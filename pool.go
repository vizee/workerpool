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

var entrypool = sync.Pool{
	New: func() interface{} {
		return &taskentry{}
	},
}

// Pool 提供了一个动态数量的 worker 池
type Pool struct {
	mu         sync.Mutex
	inited     int32
	taskcnt    int32
	head       *taskentry
	tail       *taskentry
	running    int
	idle       int
	MaxRunning int // 最大运行中 worker 数量
	MaxIdle    int // 最大空闲 worker 数量
	cond       sync.Cond
}

func (p *Pool) onceInit() {
	p.mu.Lock()
	if atomic.LoadInt32(&p.inited) == 0 {
		p.cond.L = &p.mu
		atomic.StoreInt32(&p.inited, 1)
	}
	p.mu.Unlock()
}

func (p *Pool) workerproc() {
	if atomic.LoadInt32(&p.inited) == 0 {
		p.onceInit()
	}

	for {
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
		// pop 任务
		e := p.head
		p.head = e.next
		p.mu.Unlock()

		e.t.Run()

		atomic.AddInt32(&p.taskcnt, -1)

		// 这两行代码不需要, GC 第一步直接清理 sync.Pool
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
	// 先增加任务数
	atomic.AddInt32(&p.taskcnt, 1)

	e := entrypool.Get().(*taskentry)
	e.t = t
	wakeidle := false

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

// TaskCount 返回 pending+running 的任务数量
func (p *Pool) TaskCount() int32 {
	return atomic.LoadInt32(&p.taskcnt)
}
