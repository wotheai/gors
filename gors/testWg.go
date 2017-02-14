package main

import (
	"fmt"
	"neo/gors/protocol"
)

func main() {
	t := protocol.NewTest()

	for i := 0; i < 10; i++ {
		t.TWaitGroup()
		go protocol.HandlerTest(t, i)
		fmt.Println("this is main function.")
	}

	t.TWait()
}
