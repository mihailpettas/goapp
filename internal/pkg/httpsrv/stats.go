package httpsrv

import (
	"log"
	"sync"
)

type sessionStats struct {
	id   string
	sent int64
}

type statsManager struct {
	sessions map[string]*sessionStats
	mu       sync.RWMutex
}

func newStatsManager() *statsManager {
	return &statsManager{
		sessions: make(map[string]*sessionStats),
	}
}

func (sm *statsManager) increment(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if stats, exists := sm.sessions[id]; exists {
		stats.sent++
	} else {
		sm.sessions[id] = &sessionStats{
			id:   id,
			sent: 1,
		}
	}
}

func (sm *statsManager) getStats(id string) *sessionStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	if stats, exists := sm.sessions[id]; exists {
		return &sessionStats{
			id:   stats.id,
			sent: stats.sent,
		}
	}
	return nil
}

func (sm *statsManager) removeStats(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if stats, exists := sm.sessions[id]; exists {
		log.Printf("session %s has received %d messages\n", stats.id, stats.sent)
		delete(sm.sessions, id)
	}
}

func (s *Server) initStats() {
	s.stats = newStatsManager()
}

func (s *Server) incStats(id string) {
	s.stats.increment(id)
}

func (s *Server) removeStats(id string) {
	s.stats.removeStats(id)
}