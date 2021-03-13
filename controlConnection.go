package main

import (
	"fmt"
	"net/textproto"
	"net"
	"io"
)

var command_handlers map[string] func(FtpConnection, string) error = map[string] func(FtpConnection, string) error{
	"QUIT": quit,
	"PORT": nil,
	"PASV": passive,
	"LIST": nil,
	"RETR": nil,
	"STOR": nil,
	"MKD" : not_implemented,
	"PWD" : not_implemented, // TODO maybe implement it
}

var ip_addr net.IP

var available_ports chan uint16 = make(chan uint16, 50)

func init_connectors() error {

	// adds all available ports to a "quere"/channel
	for i := uint16(12040); i <= 12090; i++ {
		available_ports <- i
	}

	// gets all addresses of all Interfaces
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return err
	}

	// and gets an IPv4 addr
	for _, addr := range addrs {
		switch v := addr.(type) {
			case *net.IPNet:
			case *net.IPAddr:
				ip_addr = v.IP
		}
		if ip_addr == nil || ip_addr.IsLoopback() {
			continue
		}

		ip_addr = ip_addr.To4()
		if ip_addr == nil { // not an IPv4 addr
			continue
		}
		break
	}
	return nil
}

type FtpConnection struct {
	control *textproto.Conn
	data *net.Conn

	current_dir string

	port uint16
	data_channel chan net.Conn
	is_passive bool

	fs FileSystem
}

func fromConn(conn io.ReadWriteCloser) FtpConnection {
	var ftp FtpConnection

	ftp.control = textproto.NewConn(conn)
	ftp.data_channel = make(chan net.Conn, 1)

	return ftp
}

func (ftp FtpConnection) handle() error {
	var command string
	var args string

	for command != "QUIT" {
		_, err := fmt.Fscanf(ftp.control.R, "%s %s\r\n", &command, &args)
		if err != nil {
			return err
		}

		command_handlers[command](ftp, args)
	}

	return nil
}

func quit(ftp FtpConnection, _ string) error {
	return nil
}

func passive(ftp FtpConnection, _ string) error {
	ftp.is_passive = true

	go func() {
		port := <- available_ports

		ftp.control.Cmd("227 Entering Passive Mode (%d,%d,%d,%d,%d,%d).",
			ip_addr[0],
			ip_addr[1],
			ip_addr[2],
			ip_addr[3],
			port >> 8,
			port & 256,
		)

		err := listen_for_passive(ftp.data_channel, port)
		if err != nil {
			fmt.Println("Couldn't listen on port ", port, " because of ", err.Error())
		}

		available_ports <- port
	}()

	return nil
}

func listen_for_passive(conn_chan chan net.Conn, port uint16) error {
	listener, err := net.Listen("tcp", fmt.Sprint(":", port))
	if err != nil {
		return err
	}

	conn, err := listener.Accept()
	if err != nil {
		return err
	}

	err = listener.Close()

	available_ports <- port
	conn_chan <- conn
	return err
}

func not_implemented(ftp FtpConnection, _ string) error {
	_, err := ftp.control.W.WriteString("202 Command not implemented, superfluous at this site.")
	return err
}
