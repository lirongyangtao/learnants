package pool

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"
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
	works       workersContainer
	workerCache sync.Pool
	cfg         *PoolConfig
	cond        *sync.Cond
	config      *PoolConfig
}

func NewPool(cap int) *Pool {
	if cap <= 0 {
		panic("not equal 0")
	}
	p := &Pool{
		workerCache: sync.Pool{
			New: func() any {
				return newGoWorker()
			},
		},
		capacity: cap,
		works:    newWorkerLoopQueue(cap),
		config: &PoolConfig{
			ExpireDuration: 10,
		},
	}
	p.cond = sync.NewCond(&p.lock)
	go p.CleanPeriodically(context.Background())
	go p.Monitor()
	return p
}

func (p *Pool) CleanPeriodically(ctx context.Context) {
	timer := time.NewTicker(time.Duration(p.config.ExpireDuration) * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
		case now := <-timer.C:
			if p.IsClosed() {
				return
			}
			p.lock.Lock()
			expires := p.works.getExpires(now.Unix() - int64(p.config.ExpireDuration))
			p.lock.Unlock()
			if len(expires) > 0 {
				log.Printf("CleanPeriodically nums:%v", len(expires))
			}

			for _, v := range expires {
				v.task <- nil
			}

			if p.Running() == 0 || (p.Waiting() > 0 && p.Free() > 0) {
				p.cond.Broadcast()
			}
		}

	}
}
func (p *Pool) Waiting() int {
	return int(atomic.LoadInt32(&p.waiting))
}
func (p *Pool) Running() int {
	return int(atomic.LoadInt32(&p.running))
}

func (p *Pool) Submit(task func()) error {
	if p.IsClosed() {
		return ErrClosed
	}
	w := p.getWorker()
	if w == nil {
		return ErrOverLoad
	}
	w.task <- task
	return nil
}

func (p *Pool) Free() int {
	c := p.Cap()
	if c < 0 {
		return -1
	}
	return c - p.Running()
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
		worker.pool = p
		go worker.Run()
	}
	p.lock.Lock()
	w := p.works.pop()
	if w != nil {
		p.lock.Unlock()
		w = worker
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
	if worker = p.works.pop(); worker == nil {
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
	if (p.Waiting() != 0 && p.Free() > 0) || p.Running() == 0 || p.IsClosed() {
		p.cond.Broadcast()
	}
	worker.recycleTime = time.Now().Unix()
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.IsClosed() {
		return false
	}
	err := p.works.offer(worker)
	if err != nil {
		log.Printf("error:%v ", err)
		return false
	}
	p.cond.Signal()
	return true
}

func (p *Pool) addRunning(num int32) {
	atomic.AddInt32(&p.running, num)
}

func (p *Pool) addWaiting(num int32) {
	atomic.AddInt32(&p.waiting, num)
}

func (p *Pool) Monitor() {
	timer := time.NewTicker(1 * time.Second)
	defer timer.Stop()
	for {
		if p.IsClosed() {
			return
		}
		select {
		case <-timer.C:
			log.Printf("waiters:%v   running:%v   ", p.Waiting(), p.Running())
		}

	}

}
