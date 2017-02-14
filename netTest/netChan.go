package main

import (
	"fmt"
	"io"
	"net"
	"os"
)

type cn struct {
	c    net.Conn
	flow map[string]net.Conn
}

func NewCn() *cn {
	return &cn{}
}

var mana *cn

func main() {
	mana = NewCn()

	l, err := net.Listen("tcp", ":8080")
	checkErr(err)

	buff := make([]byte, 3)
	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println(err)
		}
		n, err := c.Read(buff)
		if err != nil && err == io.EOF {
			fmt.Println(err)
		}

		if string(buff[:n]) == "ser" {
			go serHandler(c)
		} else if string(buff[:n]) == "cli" {
			go cliHandler(c)
		} else {
			fmt.Println("agent error.")
			os.Exit(1)
		}
	}
}

func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
}

func handler(c net.Conn) {

	b := make([]byte, 1024)
	for {
		n, err := c.Read(b)
		if err != nil && err == io.EOF {
			fmt.Println(err)
			break
		}
		fmt.Println(string(b[:n]))
	}
	defer c.Close()

}

func serHandler(c net.Conn) {
	mana.c = c
	mana.flow = make(map[string]net.Conn)
	b := make([]byte, 1024)
	for {
		n, err := c.Read(b)
		if err != nil && err == io.EOF {
			fmt.Println(err)
			break
		}
		fmt.Println(string(b[:n]))
		if mana.flow["ser"] != nil {
			mana.flow["ser"].Write(b)
		}
	}
	defer c.Close()
}
func cliHandler(c net.Conn) {
	mana.flow["ser"] = c
	defer c.Close()
}
