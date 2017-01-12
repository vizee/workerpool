package workerpool

import (
	"sync"
	"sync/atomic"
)

// Task 接口封装了任务执行所需的 Run 方法
type Task interface {
	Run()
}

type taskentry struct {
	t    Task
	next *taskentry
}

// Pool 提供了一个动态数量的 worker 池
type Pool struct {
	mu         sync.Mutex
	running    int
	idle       int
	MaxRunning int // 最大运行中 worker 数量
	MaxIdle    int //最大空闲 worker 数量
	head       *taskentry
	tail       *taskentry
	state      int32
	taskcnt    uint32
	cond       sync.Cond
}

var entrypool = sync.Pool{
	New: func() interface{} {
		return new(taskentry)
	},
}

func (p *Pool) workerproc() {
	// once init
	if p.state == 0 {
		p.mu.Lock()
		if p.state == 0 {
			p.cond.L = &p.mu
			p.state = 1
		}
		p.mu.Unlock()
	}

	for {
		// 从队列获取任务
		p.mu.Lock()
		for p.head == nil {
			// 如果空闲 worker 足够, 其余 worker 退出, 否则进入等待
			if p.idle < p.MaxIdle {
				p.idle++
			} else {
				p.running--
				p.mu.Unlock()
				return
			}
			p.cond.Wait()
		}
		e := p.head
		p.head = e.next
		p.taskcnt--
		p.mu.Unlock()

		// just do it
		e.t.Run()

		e.t = nil
		e.next = nil
		entrypool.Put(e)
	}
}

// Put 添加一个任务 t 到任务队列并且调度 worker
func (p *Pool) Put(t Task) {
	if t == nil {
		return
	}
	e := entrypool.Get().(*taskentry)
	e.t = t
	wakeidle := false

	// 先增加任务数, 如果先 push
	p.mu.Lock()
	// 把任务加到队列
	if p.head == nil {
		p.head = e
		p.tail = e
	} else {
		p.tail.next = e
		p.tail = e
	}
	p.taskcnt++

	// 调度 worker
	if p.idle > 0 {
		p.idle--
		wakeidle = true
	} else if p.running < p.MaxRunning {
		p.running++
		go p.workerproc()
	}
	p.mu.Unlock()

	if wakeidle {
		p.cond.Signal()
	}
}

func (p *Pool) TaskCount() uint32 {
	return atomic.LoadUint32(&p.taskcnt)
}
