package pool

import (
	"log"
)

type workersContainer interface {
	len() int
	isEmpty() bool
	offer(worker *goWorker) error
	pop() *goWorker
	getExpires(expireTime int64) []*goWorker
	clean()
}

type goWorker struct {
	pool        *Pool
	task        chan func()
	recycleTime int64
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
				log.Printf("task go away")
				return
			}
			task()
			if ok := g.pool.PutWorker(g); !ok {
				return
			}
		}
	}()
}
