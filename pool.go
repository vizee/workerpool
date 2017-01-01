package workerpool

import (
	"runtime"
	"sync"
)

type Task interface {
	Run()
}

type taskentry struct {
	t    Task
	next *taskentry
}

type Pool struct {
	mu         sync.Mutex
	running    int
	idle       int
	maxRunning int
	maxIdle    int
	head       *taskentry
	tail       *taskentry
	cond       sync.Cond
}

var tepool = sync.Pool{
	New: func() interface{} {
		return new(taskentry)
	},
}

func (p *Pool) workerproc() {
	for {
		p.mu.Lock()
		for p.head == nil {
			// 如果队列是空的, 增加空闲计数
			if p.idle < p.maxIdle {
				p.idle++
			} else {
				// 如果空闲 worker 足够, 其余 worker 退出
				p.running--
				p.mu.Unlock()
				return
			}
			p.cond.Wait()
		}
		// 从队列获取任务
		e := p.head
		p.head = e.next
		p.mu.Unlock()

		e.t.Run()

		e.t = nil
		e.next = nil
		tepool.Put(e)
	}
}

// Put 添加一个任务
func (p *Pool) Put(t Task) {
	if t == nil {
		return
	}
	e := tepool.Get().(*taskentry)
	e.t = t
	pulse := false

	p.mu.Lock()
	// 把任务加到队列
	if p.head == nil {
		p.head = e
		p.tail = e
	} else {
		p.tail.next = e
		p.tail = e
	}

	// 调度 worker
	if p.idle > 0 {
		p.idle--
		pulse = true
	} else if p.running < p.maxRunning {
		p.running++
		go p.workerproc()
		pulse = true
	}
	p.mu.Unlock()

	if pulse {
		p.cond.Signal()
	}
}

func New(maxIdle, maxRunning int) *Pool {
	if maxRunning <= 0 {
		maxRunning = runtime.NumCPU()
	}
	p := &Pool{
		maxIdle:    maxIdle,
		maxRunning: maxRunning,
	}
	p.cond.L = &p.mu
	return p
}
