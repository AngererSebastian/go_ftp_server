package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var INVALID_PATH error = errors.New("invalid path")
var CANT_ACCESS_FILE error = errors.New("Can't access the file or directory")

var DEFAULT_PREFIX = os.Getenv("HOME") + "/.local/share/go_ftp_server"
var PREFIX string = func() string {
	path := os.Getenv("FTP_PREFIX")

	if path == "" {
		return DEFAULT_PREFIX
	}

	path, err := filepath.Abs(path)

	if err != nil {
		path = DEFAULT_PREFIX
	}
	return path
}()

type FileSystem struct {
	current_dir string
}

func NewFs() FileSystem {
	fmt.Println("Prefix is ", PREFIX)
	return FileSystem {
		current_dir: PREFIX,
	}
}

func (fs FileSystem) list(w io.Writer, path string) error {
	files, err := os.ReadDir(path)
	if err != nil {
		return CANT_ACCESS_FILE
	}

	for _, file := range files {
		fmt.Println(file.Name())
		fmt.Fprint(w, file.Name(), "\r\n")
	}

	return nil
}

func (fs FileSystem) retrieve_file(w io.Writer, path string, is_binary bool) error {
	file, err := os.Open(path)
	if err != nil {
		return CANT_ACCESS_FILE
	}

	if is_binary {
		fmt.Println("retrieving binary file")
		_, err = io.Copy(w, file)
		return err
	} else {
		reader := bufio.NewReader(file)

		var err error
		var line string

		for err != io.EOF {
			line, err = reader.ReadString('\n')
			if err != nil && err != io.EOF {
				return err
			} else if err == nil {
				line = line[:len(line) - 1] // remove the \n at the end
			}

			fmt.Fprint(w, line, "\r\n")
		}
		return nil
	}
}

func (fs FileSystem) store_file(r io.Reader, path string, is_binary bool) error {
	file, err := os.Create(path)
	if err != nil {
		fmt.Println("test", err.Error())
		return CANT_ACCESS_FILE
	}

	if is_binary {
		_, err = io.Copy(file, r)
		return err
	} else {
		scanner := bufio.NewScanner(r)

		for scanner.Scan() {
			fmt.Fprintln(file, scanner.Text())
		}

		return scanner.Err()
	}
}

func (fs FileSystem) proccess_path(path string) (string, error) {
	var pre string
	if path == "" {
		path = fs.current_dir
	} else if path[0] == '/' {
		pre = PREFIX
		path = path[1:] //remove the first / TODO: check for more / at the beginning
	} else {
		pre = fs.current_dir
	}

	path = filepath.Join(pre, path)
	path, err := filepath.Abs(path)


	if !strings.HasPrefix(path, PREFIX) {
		return "", INVALID_PATH
	}

	fmt.Println("Accessing file: ", path)
	if err != nil {
		fmt.Println("test2", err.Error())
	}

	return path, err
}
