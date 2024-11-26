package watcher

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Watcher struct {
	id          string
	inCh        chan string
	outCh       chan *Counter
	counter     *Counter
	counterLock *sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	running     sync.WaitGroup
}

func New() *Watcher {
	ctx, cancel := context.WithCancel(context.Background())
	w := &Watcher{
		id:          uuid.NewString(),
		inCh:        make(chan string, 1),
		outCh:       make(chan *Counter, 1),
		counter:     &Counter{Iteration: 0},
		counterLock: &sync.RWMutex{},
		ctx:         ctx,
		cancel:      cancel,
		running:     sync.WaitGroup{},
	}
	return w
}

func (w *Watcher) Start() error {
	w.running.Add(1)
	go w.mainLoop()
	return nil
}

func (w *Watcher) mainLoop() {
	defer w.running.Done()
	defer close(w.outCh)
	defer close(w.inCh)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case str := <-w.inCh:
			if str == "" {
				continue
			}
			w.counterLock.Lock()
			w.counter.Iteration++
			w.counterLock.Unlock()

			select {
			case w.outCh <- w.counter:
			case <-w.ctx.Done():
				return
			case <-ticker.C:
				continue
			}
		}
	}
}

func (w *Watcher) Stop() {
	w.cancel()
	w.running.Wait()
}

func (w *Watcher) GetWatcherId() string {
	return w.id
}

func (w *Watcher) Send(str string) {
	select {
	case w.inCh <- str:
	case <-w.ctx.Done():
	default:
	}
}

func (w *Watcher) Recv() <-chan *Counter {
	return w.outCh
}

func (w *Watcher) ResetCounter() {
	w.counterLock.Lock()
	defer w.counterLock.Unlock()

	w.counter.Iteration = 0

	select {
	case w.outCh <- w.counter:
	case <-w.ctx.Done():
	default:
	}
}