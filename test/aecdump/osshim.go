package aecdump

import (
	"io"
	"os"
)

func osCreateImpl(path string) (io.WriteCloser, error) { return os.Create(path) }
func osOpenImpl(path string) (io.ReadCloser, error)    { return os.Open(path) }
