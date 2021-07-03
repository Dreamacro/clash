package vmess

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/Dreamacro/clash/common/pool"
)

const (
	lenSize   = 2
	chunkSize = 1 << 14   // 2 ** 14 == 16 * 1024
	maxSize   = 17 * 1024 // 2 + chunkSize + aead.Overhead()
)

type chunkReader struct {
	io.Reader
	buf     []byte
	sizeBuf []byte
	offset  int
}

func newChunkReader(reader io.Reader) *chunkReader {
	return &chunkReader{Reader: reader, sizeBuf: make([]byte, lenSize)}
}

func newChunkWriter(writer io.WriteCloser) *chunkWriter {
	return &chunkWriter{Writer: writer}
}

func (cr *chunkReader) Read(b []byte) (int, error) {
	if cr.buf != nil {
		n := copy(b, cr.buf[cr.offset:])
		cr.offset += n
		if cr.offset == len(cr.buf) {
			pool.Put(cr.buf)
			cr.buf = nil
		}
		return n, nil
	}

	_, err := io.ReadFull(cr.Reader, cr.sizeBuf)
	if err != nil {
		return 0, err
	}

	size := int(binary.BigEndian.Uint16(cr.sizeBuf))
	if size > maxSize {
		return 0, errors.New("buffer is larger than standard")
	}

	if len(b) >= size {
		_, err := io.ReadFull(cr.Reader, b[:size])
		if err != nil {
			return 0, err
		}

		return size, nil
	}

	buf := pool.Get(size)
	_, err = io.ReadFull(cr.Reader, buf)
	if err != nil {
		pool.Put(buf)
		return 0, err
	}
	n := copy(b, buf)
	cr.offset = n
	cr.buf = buf
	return n, nil
}

type chunkWriter struct {
	io.Writer
}

func (cw *chunkWriter) Write(b []byte) (n int, err error) {
	buf := pool.Get(pool.RelayBufferSize)
	defer pool.Put(buf)
	length := len(b)
	for {
		if length == 0 {
			break
		}
		readLen := chunkSize
		if length < chunkSize {
			readLen = length
		}
		payloadBuf := buf[lenSize : lenSize+chunkSize]
		copy(payloadBuf, b[n:n+readLen])

		binary.BigEndian.PutUint16(buf[:lenSize], uint16(readLen))
		_, err = cw.Writer.Write(buf[:lenSize+readLen])
		if err != nil {
			break
		}
		n += readLen
		length -= readLen
	}
	return
}
