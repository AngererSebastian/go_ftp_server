package main

import (
	"fmt"
	"net/textproto"
	"net"
)

var command_handlers map[string] func(*FtpConnection, string) error =
	map[string] func(*FtpConnection, string) error{
		"USER": user,
		"QUIT": quit,
		"SYST": syst,
		"PORT": port,
		"PASV": passive,
		"EPSV": nil
		"EPRT": nil // TODO: https://tools.ietf.org/html/rfc2428
		"LIST": list,
		"RETR": nil,
		"STOR": nil,
		"MKD" : not_implemented,
		"PWD" : not_implemented, // TODO maybe implement it
		"TYPE": not_implemented,
		"MODE": not_implemented,
		"STRU": not_implemented,
		"FEAT": not_implemented,
}

var available_ports chan uint16 = make(chan uint16, 50)

func init_connectors() error {

	// adds all available ports to a "quere"/channel
	for i := uint16(12040); i <= 12090; i++ {
		available_ports <- i
	}

	return nil
}

type ConnectionWithError struct {
	err error
	conn net.Conn
}

type FtpConnection struct {
	control *textproto.Conn

	client_addr net.TCPAddr
	local_ip_addr net.IP // our ip addr
	/// if passive a connection will be send through here
	data_channel chan ConnectionWithError
	is_passive bool

	// a quit has been issued
	is_quitting bool

	// the filesystem (currently not needed)
	fs FileSystem
}

func fromConn(conn net.Conn) FtpConnection {
	var ftp FtpConnection

	ftp.control = textproto.NewConn(conn)
	ftp.data_channel = make(chan ConnectionWithError, 1)
	ftp.is_passive = false
	ftp.is_quitting = false

	local_addr := conn.LocalAddr().(*net.TCPAddr)
	ftp.local_ip_addr = local_addr.IP

	fmt.Println("Connected to ", conn.RemoteAddr())


	return ftp
}

func (ftp *FtpConnection) handle() error {
	var command string
	var args string

	ftp.control.Cmd("220 Service ready for new user.")

	for !ftp.is_quitting {
		n, err := fmt.Fscanf(ftp.control.R, "%s %s\r\n", &command, &args)
		if err != nil  {
			if n == 1 {
				args = ""
			} else {
				return err
			}
		}

		fmt.Println("got ", command, " ", args)

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
			port & 0xFF,
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

func syst(ftp *FtpConnection, _ string) error {
	ftp.control.Cmd("215 UNIX system type.")

	return nil
}

func list(ftp *FtpConnection, args string) error {
	go func() {
		data, err := ftp.open_data_conn()
		if err != nil {
			return
		}

		result, err := ftp.fs.list(args)
		if err == nil {
		} else if err == INVALID_PATH {
		}

		data.Write([]byte(result))

	}()

	return nil
}

func listen_for_passive(ftp *FtpConnection, port uint16) error {
	listener, err := net.Listen("tcp", fmt.Sprint(":", port))
	if err != nil {
		return err
	}

	for !ftp.is_quitting {
		conn, err := listener.Accept()

		available_ports <- port
		ftp.data_channel <- ConnectionWithError {
			err: err,
			conn: conn,
		}
	}

	err = listener.Close()
	return err
}

func (ftp *FtpConnection) open_data_conn() (net.Conn, error) {
	var conn net.Conn
	var err error

	if ftp.is_passive {
		tmp := <- ftp.data_channel
		conn = tmp.conn
		err = tmp.err
	} else {
		conn, err = net.Dial("tcp", ftp.client_addr.String())
	}

	if err != nil {
		ftp.control.Cmd("425 Can't open data connection.")
	}

	return conn, err
}

func not_implemented(ftp *FtpConnection, _ string) error {
	_, err := ftp.control.W.WriteString("502 Command not implemented, superfluous at this site.")
	return err
}
