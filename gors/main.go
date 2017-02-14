package main

import (
	"fmt"
	"neo/gors/protocol"
	"net"
	"os"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	l, err := net.Listen("tcp", ":1935")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	acceptListen(l)
}

func acceptListen(l net.Listener) {
	for l != nil {
		c, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		go handler(c)
	}

}

func handler(c net.Conn) {

	v := protocol.NewRtmpconnection(c)
	v.SetRtmpConnectionWaitGroup()
	go protocol.HandlerRtmpConnection(v)

	v.WaitRtmpConnection()

}
