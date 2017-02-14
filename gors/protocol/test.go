package protocol

import (
	"fmt"
	"sync"
)

type test struct {
	a  int
	b  int
	wg *sync.WaitGroup
}

func NewTest() *test {
	return &test{wg: &sync.WaitGroup{}}
}

func (t *test) SetA(a int) {
	t.a = a
}

func (t *test) GetA() int {
	return t.a
}

func (v *test) TWaitGroup() {
	v.wg.Add(1)
}

func (v *test) TWait() {
	v.wg.Wait()
}

func HandlerTest(t *test, i int) {
	//t.wg.Add(1)
	for j := 0; j < 10; j++ {
		fmt.Println("i = ", i, "t = ", j)
	}

	defer t.wg.Done()
}
