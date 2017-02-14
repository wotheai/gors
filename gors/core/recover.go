package core

import (
	"fmt"
	"runtime/debug"
)

func Recover(f func() error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("goroutine abort with", r)
			fmt.Println(string(debug.Stack()))
		}
	}()

	err := f()
	if err != nil {
		fmt.Println(err)
		fmt.Println("rtmpConnection error exit.")
	}
}
