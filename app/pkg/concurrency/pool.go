package concurrency

import (
	"context"
	"sync"
)

type WorkerPool struct {
	workers   int
	taskQueue chan func()
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewWorkerPool(workers, queueSize int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	pool := &WorkerPool{
		workers:   workers,
		taskQueue: make(chan func(), queueSize),
		ctx:       ctx,
		cancel:    cancel,
	}
	
	pool.start()
	return pool
}

func (p *WorkerPool) start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

func (p *WorkerPool) worker() {
	defer p.wg.Done()
	
	for {
		select {
		case task, ok := <-p.taskQueue:
			if !ok {
				return
			}
			task()
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *WorkerPool) Submit(task func()) bool {
	select {
	case p.taskQueue <- task:
		return true
	case <-p.ctx.Done():
		return false
	default:
		return false
	}
}

func (p *WorkerPool) Stop() {
	close(p.taskQueue)
	p.cancel()
	p.wg.Wait()
}

type SafeCounter struct {
	mu    sync.RWMutex
	count int64
}

func (c *SafeCounter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
}

func (c *SafeCounter) Get() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.count
}

type SafeMap struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

func NewSafeMap() *SafeMap {
	return &SafeMap{
		data: make(map[string]interface{}),
	}
}

func (m *SafeMap) Set(key string, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
}

func (m *SafeMap) Get(key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[key]
	return val, ok
}

func (m *SafeMap) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
}
