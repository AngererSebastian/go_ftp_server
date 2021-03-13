package main

import (
	"os"
	"strings"
	"errors"
	"fmt"
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

func (fs FileSystem) list(path string) (string, error) {
	path, err := fs.proccess_path(path)
	if err != nil {
		return "", INVALID_PATH
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return "", CANT_ACCESS_DIR
	}

	var output strings.Builder

	for _, file := range files {
		output.WriteString(
			fmt.Sprint(file.Type().String(), file.Name()),
		)
	}

	fmt.Println(output.String())
	return output.String(), nil
}

func (fs FileSystem) proccess_path(path string) (string, error) {
	var pre string
	if path[0] == '/' {
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
