package httpsrv

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
)

type Server struct {
	strChan      <-chan string
	server       *http.Server
	watchers     map[string]*watcher.Watcher
	watchersLock *sync.RWMutex
	stats        *statsManager
	secureCookie *securecookie.SecureCookie
	ctx          context.Context
	cancel       context.CancelFunc
	running      sync.WaitGroup
}

func New(strChan <-chan string) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	
	hashKey := make([]byte, 32)
	blockKey := make([]byte, 32)
	if _, err := rand.Read(hashKey); err != nil {
		panic(err)
	}
	if _, err := rand.Read(blockKey); err != nil {
		panic(err)
	}

	s := &Server{
		strChan:      strChan,
		watchers:     make(map[string]*watcher.Watcher),
		watchersLock: &sync.RWMutex{},
		secureCookie: securecookie.New(hashKey, blockKey),
		ctx:          ctx,
		cancel:       cancel,
	}
	
	s.initStats()
	return s
}

func (s *Server) Start() error {
	r := mux.NewRouter()
	
	r.Use(s.csrfMiddleware)
	r.Use(s.securityHeadersMiddleware)

	for _, route := range s.myRoutes() {
		r.Handle(route.Pattern, route.HFunc).
			Methods(route.Method).
			Name(route.Name)
		
		if route.Queries != nil {
			r.Queries(route.Queries...)
		}
	}

	s.server = &http.Server{
		Addr:              "localhost:8080",
		Handler:           handlers.CombinedLoggingHandler(os.Stdout, r),
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	s.running.Add(1)
	go func() {
		defer s.running.Done()
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v\n", err)
		}
	}()

	s.running.Add(1)
	go s.mainLoop()

	return nil
}

func (s *Server) Stop() {
	s.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v\n", err)
	}

	s.running.Wait()
}

func (s *Server) mainLoop() {
	defer s.running.Done()

	for {
		select {
		case str := <-s.strChan:
			s.notifyWatchers(str)
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Server) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.Header.Get("Upgrade") == "websocket" {

			origin := r.Header.Get("Origin")
			if !s.isValidOrigin(origin) {
				http.Error(w, "Invalid origin", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		if r.Method != "GET" {
			token := r.Header.Get("X-CSRF-Token")
			cookie, err := r.Cookie("csrf_token")
			
			if err != nil || token == "" || token != cookie.Value {
				http.Error(w, "Invalid CSRF token", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) isValidOrigin(origin string) bool {
	allowedOrigins := map[string]bool{
		"http://localhost:8080": true,
		"https://localhost:8080": true,
	}
	return allowedOrigins[origin]
}

func (s *Server) generateCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

func (s *Server) setCSRFToken(w http.ResponseWriter) string {
	token := s.generateCSRFToken()
	if token == "" {
		return ""
	}

	encoded, err := s.secureCookie.Encode("csrf_token", token)
	if err != nil {
		return ""
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    encoded,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   3600, // 1 hour
	})

	return token
}