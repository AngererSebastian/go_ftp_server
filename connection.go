package main

import (
	"errors"
	"fmt"
	"net"
	"net/textproto"
	"strings"
)

const NUMBER_PORT_AVAIL = 50

var command_handlers map[string] func(*FtpConnection, string) error =
	map[string] func(*FtpConnection, string) error{
		"USER": user,
		"QUIT": quit,
		"SYST": syst,
		"PORT": port,
		"PASV": passive,
		"LIST": list,
		"RETR": retrieve,
		"STOR": store,
		"FEAT": feat,
		"PWD" : print_working_dir,
}

var available_ports chan uint16 = make(chan uint16, NUMBER_PORT_AVAIL)

func init_connectors() error {

	// adds all available ports to a "quere"/channel
	for i := uint16(12040); i < 12040 + NUMBER_PORT_AVAIL; i++ {
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

	// a quit has been issued
	is_quitting bool
	// should it open an active or passive data connection
	is_passive bool
	// binary file transfer mode (only ascii and bin is implemented)
	is_binary bool

	// the filesystem (currently not needed)
	fs FileSystem
	// for logging
	remote net.Addr
}

func fromConn(conn net.Conn) FtpConnection {
	var ftp FtpConnection

	ftp.control = textproto.NewConn(conn)
	ftp.data_channel = make(chan ConnectionWithError, 1)
	ftp.is_passive = false
	ftp.is_quitting = false
	ftp.is_binary = false

	local_addr := conn.LocalAddr().(*net.TCPAddr)
	ftp.local_ip_addr = local_addr.IP
	ftp.remote = conn.RemoteAddr()
	ftp.fs = NewFs()

	fmt.Println("Connected to ", conn.RemoteAddr())


	return ftp
}

func (ftp *FtpConnection) handle() error {
	defer ftp.control.Close()
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

		fmt.Printf("%s -> %s %s\n", ftp.remote.String(), command, args)

		handler := command_handlers[command]

		if handler == nil {
			not_implemented(ftp, args)
		} else {
			handler(ftp, args)
		}
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
		fmt.Printf("%s -> got port %d\n", ftp.remote.String(), port)

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
	var addr []byte = make([]byte, 4)
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

func print_working_dir(ftp *FtpConnection, _ string) error {
	_, err := ftp.control.Cmd("257 \"%s\" is the current dir", ftp.fs.current_dir)
	return err
}

func list(ftp *FtpConnection, args string) error {
	path, err := check_file(ftp, args)
	if err != nil {
		return err
	}

	go func() {
		data, err := ftp.open_data_conn()
		defer data.Close()

		if err != nil {
			return
		}

		err = ftp.fs.list(data, path)
		if err == nil {
		} else if err == INVALID_PATH {
			ftp.control.Cmd("550 %s", err.Error())
		}

		ftp.control.Cmd("226 Closing data connection")
	}()

	return nil
}

func retrieve(ftp *FtpConnection, args string) error {
	path, err := check_file(ftp, args)
	if err != nil {
		return err
	}

	go func() {
		data_conn, err := ftp.open_data_conn()
		defer data_conn.Close()

		if err != nil {
			return
		}

		err = ftp.fs.retrieve_file(data_conn, path, ftp.is_binary)
		if err == CANT_ACCESS_FILE {
			ftp.control.Cmd("550 %s", err.Error())
		} else if err != nil {
			ftp.control.Cmd("552 action aborted")
		}

		ftp.control.Cmd("226 Closing data connection")
	}()
	return nil
}

func store(ftp *FtpConnection, args string) error {
	path, err := check_file(ftp, args)
	if err != nil {
		return err
	}

	go func() {
		data_conn, err := ftp.open_data_conn()
		defer data_conn.Close()

		if err != nil {
			return
		}

		err = ftp.fs.store_file(data_conn, path, ftp.is_binary)
		if err == CANT_ACCESS_FILE {
			ftp.control.Cmd("550 %s", err.Error())
		} else if err != nil {
			ftp.control.Cmd("552 action aborted")
		}

		ftp.control.Cmd("226 Closing data connection")
	}()

	return nil
}

func listen_for_passive(ftp *FtpConnection, port uint16) error {
	listener, err := net.Listen("tcp", fmt.Sprint(":", port))
	defer listener.Close()
	if err != nil {
		return err
	}

	for !ftp.is_quitting {
		conn, err := listener.Accept()
		fmt.Println("accecpted data connection from", conn.RemoteAddr().String())

		available_ports <- port
		ftp.data_channel <- ConnectionWithError {
			err: err,
			conn: conn,
		}
	}

	return nil
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
		fmt.Println("establishing active data conn with ", ftp.client_addr.String())
	}

	if err != nil {
		ftp.control.Cmd("425 Can't open data connection.")
	} else {
		ftp.control.Cmd("150 File status okay; about to open data connection")
	}

	return conn, err
}

func not_implemented(ftp *FtpConnection, _ string) error {
	_, err := ftp.control.W.WriteString("502 Command not implemented, superfluous at this site.")
	return err
}

func feat(ftp *FtpConnection, _ string) error {
	_, err := ftp.control.Cmd("211 No Features.")
	return err
}

func check_file(ftp *FtpConnection, args string) (string, error) {
	if len(strings.Split(args, " ")) > 1 {
		ftp.control.Cmd("501 Syntax error in parameters or arguments.")
		return "", errors.New("too many arguments")
	}

	path, err := ftp.fs.proccess_path(args)
	if err != nil {
		ftp.control.Cmd("550 %s", err.Error())
	}
	return path, err
}
