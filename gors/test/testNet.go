package main

import (
	"fmt"
	"sync"
)

type a struct {
	ch chan bool
	wg *sync.WaitGroup
}

type b struct {
	ch chan bool
	wg *sync.WaitGroup
}

func newA() *a {
	return &a{ch: make(chan bool), wg: &sync.WaitGroup{}}
}

func newB() *b {
	return &b{ch: make(chan bool), wg: &sync.WaitGroup{}}
}

func main() {
	aa := newA()
	//bb :=newB()
	aa.wg.Add(1)
	go aa.handler()
	aa.wg.Wait()
	fmt.Println("all done.")
}

func (v *a) handler() {
	bb := newB()
	i := 0
	//v.wg.Add(1)

	defer v.wg.Done()

	for {
		bb.wg.Add(1)
		go bb.handler()
		i++
		fmt.Println(i)
		if i == 1000 {
			fmt.Println("end a's loop.")
			return
		}
	}

	bb.wg.Wait()
}

func (v *b) handler() {
	defer v.wg.Done()
	var i = 0
	for {
		i++
		if i == 99 {
			v.ch <- true
			continue
		}

		select {
		case <-v.ch:
			fmt.Println("get bb's chan.")
			return
		default:
			//fmt.Println("this is default.")

		}
	}
}
