package main

import (
	"fmt"
	"net"
)

func main() {
	fmt.Print("hello world\n")

	init_connectors()

	listener, err := net.Listen("tcp", ":4000")
	if err != nil {
		fmt.Println(err)
		return
	}

	defer listener.Close()

	for {
		conn, err := listener.Accept();
		if err != nil {
			fmt.Println(err)
			return
		}

		ftp_conn := fromConn(conn)
		go ftp_conn.handle()
	}
}
