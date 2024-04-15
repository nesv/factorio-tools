package server

import (
	"errors"
	"io/fs"
	"os"
)

// dirExists indicates whether or not path name exists and is a directory.
func dirExists(name string) (bool, error) {
	if name == "" {
		return false, errors.New("empty path name")
	}
	info, err := os.Stat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if !info.IsDir() {
		return false, errors.New("not a directory")
	}
	return true, nil
}

// fileExists indicates whether or not path name exists and is a file.
func fileExists(name string) (bool, error) {
	if name == "" {
		return false, errors.New("empty path name")
	}
	info, err := os.Stat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if info.IsDir() {
		return false, errors.New("is a directory")
	}
	return true, nil
}
