package pool

import (
	"time"
)

type workersContainer interface {
	len() int
	isEmpty() bool
	offer(worker *goWorker) error
	pop() *goWorker
	getExpires(duration time.Duration) []*goWorker
	clean()
}

type goWorker struct {
	pool        Pool
	task        chan func()
	recycleTime time.Time
}

func newGoWorker() *goWorker {
	return &goWorker{
		task: make(chan func()),
	}
}
func (g *goWorker) Run() {
	g.pool.addRunning(1)
	go func() {
		defer func() {
			g.pool.addRunning(-1)
			g.pool.workerCache.Put(g)
			if p := recover(); p != nil {

			}

		}()
		for task := range g.task {
			if task == nil {
				return
			}
			task()
			if ok := g.pool.PutWorker(g); !ok {
				return
			}
		}
	}()
}
