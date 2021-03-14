package main

import (
	"os"
	"errors"
	"fmt"
	"io"
	"path/filepath"
)

var INVALID_PATH error = errors.New("invalid path")
var CANT_ACCESS_DIR error = errors.New("couldn't read dir")
var PREFIX string = os.Getenv("FTP_PREFIX")

type FileSystem struct {
	current_dir string
}

func NewFs() FileSystem {
	return FileSystem {
		current_dir: PREFIX,
	}
}

func (fs FileSystem) list(w io.Writer, path string) error {
	files, err := os.ReadDir(path)
	if err != nil {
		return CANT_ACCESS_DIR
	}

	for _, file := range files {
		fmt.Println(file.Name())
		fmt.Fprint(w, file.Name(), "\r\n")
	}

	return nil
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

	fmt.Println("Accessing file: ", path)

	return path, err
}
