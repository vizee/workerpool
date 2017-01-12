package workerpool

import (
	"log"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestStruct(t *testing.T) {
	rt := reflect.TypeOf(Pool{})
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		t.Logf("%-10s %4d %4d", f.Name, f.Offset, f.Type.Size())
	}
}

type sleeptask struct {
	wg *sync.WaitGroup
	i  int
}

func (t *sleeptask) Run() {
	log.Println("task", t, "run")
	time.Sleep(time.Second)
	log.Println("task", t, "done")
	t.wg.Done()
}

func TestWork(t *testing.T) {
	wg := sync.WaitGroup{}
	p := &Pool{MaxIdle: 4, MaxRunning: 8}
	wg.Add(10)
	for i := 0; i < 10; i++ {
		p.Put(&sleeptask{&wg, i})
		t.Logf("taskcnt %d", p.TaskCount())
	}
	wg.Wait()
	t.Logf("%+v", p)
	t.Logf("taskcnt %d", p.TaskCount())
}

type benchtask sync.WaitGroup

func (t *benchtask) Run() {
	(*sync.WaitGroup)(t).Done()
}

func BenchmarkWork(b *testing.B) {
	wg := sync.WaitGroup{}
	p := &Pool{MaxIdle: 4, MaxRunning: 64}
	wg.Add(b.N)
	for i := 0; i < b.N; i++ {
		p.Put((*benchtask)(&wg))
	}
	wg.Wait()
}
