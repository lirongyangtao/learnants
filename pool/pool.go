package pool

import (
	"errors"
	"sync"
	"sync/atomic"
)

var (
	ErrClosed   = errors.New("pool has closed ")
	ErrOverLoad = errors.New("pool overLoad ")
)

type Pool struct {
	capacity    int   // 容量
	running     int32 // 当前运行数量
	waiting     int32 // 等待数量
	Closed      int32
	lock        sync.Mutex
	container   workersContainer
	workerCache sync.Pool
	cfg         *PoolConfig
	cond        sync.Cond
}
type PoolConfig struct {
}

func NewPool() *Pool {
	p := &Pool{
		workerCache: sync.Pool{
			New: func() any {
				return newGoWorker()
			},
		},
	}
	go p.CleanPeriodically()
	return p
}

func (p *Pool) CleanPeriodically() {

}

func (p *Pool) Running() int {
	return int(atomic.LoadInt32(&p.running))
}

func (p *Pool) Submit(task func()) error {
	if p.IsClosed() {
		return ErrClosed
	}
	w := p.getWorker()
	if w != nil {
		return ErrOverLoad
	}
	w.task <- task
	return nil
}

func (p *Pool) Free() int {

}

func (p *Pool) Cap() int {
	return p.capacity
}

func (p *Pool) IsClosed() bool {
	return atomic.LoadInt32(&p.Closed) == 1
}

func (p *Pool) getWorker() (worker *goWorker) {
	spawnWorker := func() {
		worker = p.workerCache.Get().(*goWorker)
		go worker.Run()
	}
	p.lock.Lock()
	w := p.container.pop()
	if w != nil {
		p.lock.Unlock()
		return
	}
Retry:
	// 容量足够直接生成一个worker
	if p.capacity == 0 || p.Cap() != 0 && p.Running() < p.Cap() {
		p.lock.Unlock()
		spawnWorker()
		return
	}
	// 容量不足够
	p.addWaiting(1)
	p.cond.Wait()
	p.addWaiting(-1)
	if p.IsClosed() {
		p.lock.Unlock()
		return
	}
	var nw int
	if nw = p.Running(); nw == 0 {
		p.lock.Unlock()
		spawnWorker()
		return
	}
	if worker := p.container.pop(); worker == nil {
		if nw < p.Cap() {
			p.lock.Unlock()
			spawnWorker()
			return
		}
		goto Retry
	}
	p.lock.Unlock()
	return
}

func (p *Pool) PutWorker(worker *goWorker) bool {

	return true
}

func (p *Pool) addRunning(num int32) {
	atomic.AddInt32(&p.running, num)
}

func (p *Pool) addWaiting(num int32) {
	atomic.AddInt32(&p.waiting, num)
}
