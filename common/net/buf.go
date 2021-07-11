package net

import (
	"bufio"
	"io"
)

type BufferReadCloser struct {
	io.Closer
	*bufio.Reader
}

func NewBufferReadCloser(rc io.ReadCloser) *BufferReadCloser {
	return &BufferReadCloser{
		Closer: rc,
		Reader: bufio.NewReader(rc),
	}
}
