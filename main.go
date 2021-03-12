package main

import (
	"fmt"
	"net"
)

func main() {
	fmt.Print("hello world\n")

	listener, err := net.Listen("tcp", ":4000")
	if err != nil {
		fmt.Println(err)
		return
	}

	defer func() {
		fmt.Print("Shutting down server\n")
		listener.Close()
	}()

	for {
		conn, err := listener.Accept();
		if err != nil {
			fmt.Println(err)
			return
		}

		go fromConn(conn).handle()
	}
}
