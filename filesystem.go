package main

import (
	"os"
	"strings"
	"errors"
	"fmt"
	"path/filepath"
)

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
	files, err := os.ReadDir(path)
	if err != nil {
		return "", errors.New("couldn't read dir")
	}

	var output strings.Builder

	for _, file := range files {
		output.WriteString(
			fmt.Sprint(file.Type().String()),
		)
	}

	return output.String(), nil
}

func (fs FileSystem) proccess_path(path string) (string, error) {
	var pre string
	if path[0] == '/' {
		pre = PREFIX
	} else {
		pre = fs.current_dir
	}

	path = filepath.Join(pre, path)
	path, err := filepath.Abs(path)

	fmt.Println("Accessing file: ", path)

	return path, err
}
