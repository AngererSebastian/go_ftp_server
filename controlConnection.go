package main

import (
	"fmt"
	"net/textproto"
	"net"
)

var command_handlers map[string] func(*FtpConnection, string) error = map[string] func(*FtpConnection, string) error{
	"USER": user,
	"QUIT": quit,
	"PORT": port,
	"PASV": passive,
	"LIST": nil,
	"RETR": nil,
	"STOR": nil,
	"MKD" : not_implemented,
	"PWD" : not_implemented, // TODO maybe implement it
	"TYPE": not_implemented,
	"MODE": not_implemented,
	"STRU": not_implemented,
}

var available_ports chan uint16 = make(chan uint16, 50)

func init_connectors() error {

	// adds all available ports to a "quere"/channel
	for i := uint16(12040); i <= 12090; i++ {
		available_ports <- i
	}

	return nil
}

type FtpConnection struct {
	control *textproto.Conn

	client_addr net.TCPAddr
	local_ip_addr net.IP // our ip addr
	/// if passive a connection will be send through here
	data_channel chan net.Conn
	is_passive bool

	// a quit has been issued
	is_quitting bool

	// the filesystem (currently not needed)
	fs FileSystem
}

func fromConn(conn net.Conn) FtpConnection {
	var ftp FtpConnection

	ftp.control = textproto.NewConn(conn)
	ftp.data_channel = make(chan net.Conn, 1)
	ftp.is_passive = false
	ftp.is_quitting = false

	local_addr := conn.LocalAddr().(*net.TCPAddr)
	ftp.local_ip_addr = local_addr.IP


	return ftp
}

func (ftp *FtpConnection) handle() error {
	var command string
	var args string

	for !ftp.is_quitting {
		n, err := fmt.Fscanf(ftp.control.R, "%s %s\r\n", &command, &args)
		if err != nil  {
			if n == 1 {
				args = ""
			} else {
				return err
			}
		}

		command_handlers[command](ftp, args)
	}

	return nil
}

func quit(ftp *FtpConnection, _ string) error {
	ftp.is_quitting = true
	ftp.control.Cmd("221 Service closing control connection.")
	return nil
}

func passive(ftp *FtpConnection, _ string) error {
	ftp.is_passive = true

	go func() {
		port := <- available_ports

		ftp.control.Cmd("227 Entering Passive Mode (%d,%d,%d,%d,%d,%d).",
			ftp.local_ip_addr[0],
			ftp.local_ip_addr[1],
			ftp.local_ip_addr[2],
			ftp.local_ip_addr[3],
			port >> 8,
			port & 256,
		)

		err := listen_for_passive(ftp, port)
		if err != nil {
			fmt.Println("Couldn't listen on port ", port, " because of ", err.Error())
		}

		available_ports <- port
	}()

	return nil
}

func port(ftp *FtpConnection, args string) error {
	var addr net.IP
	var port1, port2 int

	_, err := fmt.Sscanf(args, "%d,%d,%d,%d,%d,%d",
		&addr[0],
		&addr[1],
		&addr[2],
		&addr[3],
		&port1,
		&port2,
	)
	if err != nil {
		ftp.control.Cmd("501 Syntax error in parameters or arguments.")
		return err
	}


	ftp.client_addr = net.TCPAddr{
		IP: addr,
		Port: (port1 << 8) + port2,
	}

	ftp.control.Cmd("200 Port aknowledged")
	return nil
}

func user(ftp *FtpConnection, _ string) error {
	ftp.control.Cmd("230 User logged in, proceed.")
	return nil
}

func listen_for_passive(ftp *FtpConnection, port uint16) error {
	listener, err := net.Listen("tcp", fmt.Sprint(":", port))
	if err != nil {
		return err
	}

	for !ftp.is_quitting {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		available_ports <- port
		ftp.data_channel <- conn
	}

	err = listener.Close()
	return err
}

func not_implemented(ftp *FtpConnection, _ string) error {
	_, err := ftp.control.W.WriteString("202 Command not implemented, superfluous at this site.")
	return err
}
