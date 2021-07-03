package vmess

import (
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"io"
	"sync"

	"github.com/Dreamacro/clash/common/pool"
)

type aeadWriter struct {
	io.Writer
	cipher.AEAD
	nonce [32]byte
	count uint16
	iv    []byte

	writeLock sync.Mutex
}

func newAEADWriter(w io.Writer, aead cipher.AEAD, iv []byte) *aeadWriter {
	return &aeadWriter{Writer: w, AEAD: aead, iv: iv}
}

func (w *aeadWriter) Write(b []byte) (n int, err error) {
	w.writeLock.Lock()
	buf := pool.Get(pool.RelayBufferSize)
	defer func() {
		w.writeLock.Unlock()
		pool.Put(buf)
	}()
	length := len(b)
	for {
		if length == 0 {
			break
		}
		readLen := chunkSize - w.Overhead()
		if length < readLen {
			readLen = length
		}
		payloadBuf := buf[lenSize : lenSize+chunkSize-w.Overhead()]
		copy(payloadBuf, b[n:n+readLen])

		binary.BigEndian.PutUint16(buf[:lenSize], uint16(readLen+w.Overhead()))
		binary.BigEndian.PutUint16(w.nonce[:2], w.count)
		copy(w.nonce[2:], w.iv[2:12])

		w.Seal(payloadBuf[:0], w.nonce[:w.NonceSize()], payloadBuf[:readLen], nil)
		w.count++

		_, err = w.Writer.Write(buf[:lenSize+readLen+w.Overhead()])
		if err != nil {
			break
		}
		n += readLen
		length -= readLen
	}
	return
}

type aeadReader struct {
	io.Reader
	cipher.AEAD
	nonce   [32]byte
	buf     []byte
	offset  int
	iv      []byte
	sizeBuf []byte
	count   uint16
}

func newAEADReader(r io.Reader, aead cipher.AEAD, iv []byte) *aeadReader {
	return &aeadReader{Reader: r, AEAD: aead, iv: iv, sizeBuf: make([]byte, lenSize)}
}

func (r *aeadReader) Read(b []byte) (int, error) {
	if r.buf != nil {
		n := copy(b, r.buf[r.offset:])
		r.offset += n
		if r.offset == len(r.buf) {
			pool.Put(r.buf)
			r.buf = nil
		}
		return n, nil
	}

	_, err := io.ReadFull(r.Reader, r.sizeBuf)
	if err != nil {
		return 0, err
	}

	size := int(binary.BigEndian.Uint16(r.sizeBuf))
	if size > maxSize {
		return 0, errors.New("buffer is larger than standard")
	}

	buf := pool.Get(size)
	_, err = io.ReadFull(r.Reader, buf[:size])
	if err != nil {
		pool.Put(buf)
		return 0, err
	}

	binary.BigEndian.PutUint16(r.nonce[:2], r.count)
	copy(r.nonce[2:], r.iv[2:12])

	_, err = r.Open(buf[:0], r.nonce[:r.NonceSize()], buf[:size], nil)
	r.count++
	if err != nil {
		return 0, err
	}
	realLen := size - r.Overhead()
	n := copy(b, buf[:realLen])
	if len(b) >= realLen {
		pool.Put(buf)
		return n, nil
	}

	r.offset = n
	r.buf = buf[:realLen]
	return n, nil
}
