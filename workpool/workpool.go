package workpool

import (
	"log/slog"
	"runtime/debug"
	"sync"
)

type Params struct {
	WorkersCount int
	QueueSize    int
}

type WorkPool[T any] interface {
	TryPush(task T) bool
	Close()
}

type workPool[T any] struct {
	tasks chan T
	wg    *sync.WaitGroup
}

func NewWorkPool[T any](params *Params, handle func(T)) WorkPool[T] {
	p := &workPool[T]{
		tasks: make(chan T, params.QueueSize),
		wg:    &sync.WaitGroup{},
	}

	for i := 0; i < params.WorkersCount; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for task := range p.tasks {
				func() {
					defer func() {
						if r := recover(); r != nil {
							slog.Error("panic in worker",
								"panic", r,
								"stack", string(debug.Stack()))
						}
					}()
					handle(task)
				}()
			}
		}()
	}

	return p
}

func (p *workPool[T]) TryPush(task T) bool {
	select {
	case p.tasks <- task:
		return true
	default:
		return false
	}
}

func (p *workPool[T]) Close() {
	close(p.tasks)
	p.wg.Wait()
}
