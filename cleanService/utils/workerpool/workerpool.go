package workerpool

import "sync"

type WorkerPool struct {
	workers int
	Tasks   chan func()
	wg      sync.WaitGroup
}

func NewWorkerPool(workers int) *WorkerPool {
	return &WorkerPool{
		workers: workers,
		Tasks:   make(chan func()),
	}
}
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go func() {
			defer wp.wg.Done()
			for task := range wp.Tasks {
				task()
			}
		}()
	}
}

func (wp *WorkerPool) Stop() {
	close(wp.Tasks)
	wp.wg.Wait()
}
