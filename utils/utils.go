package utils

import (
	"os"
)

func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0750)
}

func WriteFile(fn string, data []byte) error {
	return os.WriteFile(fn, data, 0660)
}
