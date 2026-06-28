//go:build windows
// +build windows

package main

import (
	"os"
	"path/filepath"
)

func lockConfig() (func(), error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	lockPath := path + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	return func() {
		file.Close()
	}, nil
}
