package http

import (
	"bufio"
	"net/http"
)

//go:linkname ReadRequest http.readRequest
func ReadRequest(b *bufio.Reader, deleteHostHeader bool) (req *http.Request, err error) {
	panic("stub!")
}
