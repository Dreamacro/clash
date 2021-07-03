package protocol

import (
	"bytes"
	"encoding/binary"
	"hash/adler32"
	"hash/crc32"
	"math/rand"
	"net"

	"github.com/Dreamacro/clash/common/pool"
	"github.com/Dreamacro/clash/transport/ssr/tools"
)

func init() {
	register("auth_sha1_v4", newAuthSHA1V4, 7)
}

type authSHA1V4 struct {
	*Base
	*authData
	iv            []byte
	hasSentHeader bool
	rawTrans      bool
}

func newAuthSHA1V4(b *Base) Protocol {
	return &authSHA1V4{Base: b, authData: &authData{}}
}

func (a *authSHA1V4) StreamConn(c net.Conn, iv []byte) net.Conn {
	p := &authSHA1V4{Base: a.Base, authData: a.next()}
	p.iv = iv
	return &Conn{Conn: c, Protocol: p}
}

func (a *authSHA1V4) PacketConn(c net.PacketConn) net.PacketConn {
	return c
}

func (a *authSHA1V4) Decode(dst, src *bytes.Buffer) error {
	if a.rawTrans {
		dst.ReadFrom(src)
		return nil
	}
	for src.Len() > 4 {
		if uint16(crc32.ChecksumIEEE(src.Bytes()[:2])&0xffff) != binary.LittleEndian.Uint16(src.Bytes()[2:4]) {
			src.Reset()
			return errAuthSHA1V4CRC32Error
		}

		length := int(binary.BigEndian.Uint16(src.Bytes()[:2]))
		if length >= 8192 || length < 7 {
			a.rawTrans = true
			src.Reset()
			return errAuthSHA1V4LengthError
		}
		if length > src.Len() {
			break
		}

		if adler32.Checksum(src.Bytes()[:length-4]) != binary.LittleEndian.Uint32(src.Bytes()[length-4:length]) {
			a.rawTrans = true
			src.Reset()
			return errAuthSHA1V4Adler32Error
		}

		pos := int(src.Bytes()[4])
		if pos < 255 {
			pos += 4
		} else {
			pos = int(binary.BigEndian.Uint16(src.Bytes()[5:7])) + 4
		}
		dst.Write(src.Bytes()[pos : length-4])
		src.Next(length)
	}
	return nil
}

func (a *authSHA1V4) Encode(buf *bytes.Buffer, b []byte) error {
	if !a.hasSentHeader {
		dataLength := getDataLength(b)

		a.packAuthData(buf, b[:dataLength])
		b = b[dataLength:]

		a.hasSentHeader = true
	}
	for len(b) > 8100 {
		a.packData(buf, b[:8100])
		b = b[8100:]
	}
	if len(b) > 0 {
		a.packData(buf, b)
	}

	return nil
}

func (a *authSHA1V4) DecodePacket(b []byte) ([]byte, error) { return b, nil }

func (a *authSHA1V4) EncodePacket(buf *bytes.Buffer, b []byte) error {
	buf.Write(b)
	return nil
}

func (a *authSHA1V4) packData(poolBuf *bytes.Buffer, data []byte) {
	dataLength := len(data)
	randDataLength := a.getRandDataLength(dataLength)
	/*
		2:	uint16 BigEndian packedDataLength
		2:	uint16 LittleEndian crc32Data & 0xffff
		3:	maxRandDataLengthPrefix (min:1)
		4:	adler32Data
	*/
	packedDataLength := 2 + 2 + 3 + randDataLength + dataLength + 4
	if randDataLength < 128 {
		packedDataLength -= 2
	}

	binary.Write(poolBuf, binary.BigEndian, uint16(packedDataLength))
	binary.Write(poolBuf, binary.LittleEndian, uint16(crc32.ChecksumIEEE(poolBuf.Bytes()[poolBuf.Len()-2:])&0xffff))
	a.packRandData(poolBuf, randDataLength)
	poolBuf.Write(data)
	binary.Write(poolBuf, binary.LittleEndian, adler32.Checksum(poolBuf.Bytes()[poolBuf.Len()-packedDataLength+4:]))
}

func (a *authSHA1V4) packAuthData(poolBuf *bytes.Buffer, data []byte) {
	dataLength := len(data)
	randDataLength := a.getRandDataLength(12 + dataLength)
	/*
		2:	uint16 BigEndian packedAuthDataLength
		4:	uint32 LittleEndian crc32Data
		3:	maxRandDataLengthPrefix (min: 1)
		12:	authDataLength
		10:	hmacSHA1DataLength
	*/
	packedAuthDataLength := 2 + 4 + 3 + randDataLength + 12 + dataLength + 10
	if randDataLength < 128 {
		packedAuthDataLength -= 2
	}

	salt := []byte("auth_sha1_v4")
	crcData := pool.Get(len(salt) + len(a.Key) + 2)
	defer pool.Put(crcData)
	binary.BigEndian.PutUint16(crcData, uint16(packedAuthDataLength))
	copy(crcData[2:], salt)
	copy(crcData[2+len(salt):], a.Key)

	key := pool.Get(len(a.iv) + len(a.Key))
	defer pool.Put(key)
	copy(key, a.iv)
	copy(key[len(a.iv):], a.Key)

	poolBuf.Write(crcData[:2])
	binary.Write(poolBuf, binary.LittleEndian, crc32.ChecksumIEEE(crcData))
	a.packRandData(poolBuf, randDataLength)
	a.putAuthData(poolBuf)
	poolBuf.Write(data)
	poolBuf.Write(tools.HmacSHA1(key, poolBuf.Bytes()[poolBuf.Len()-packedAuthDataLength+10:])[:10])
}

func (a *authSHA1V4) packRandData(poolBuf *bytes.Buffer, size int) {
	if size < 128 {
		poolBuf.WriteByte(byte(size + 1))
		tools.AppendRandBytes(poolBuf, size)
		return
	}
	poolBuf.WriteByte(255)
	binary.Write(poolBuf, binary.BigEndian, uint16(size+3))
	tools.AppendRandBytes(poolBuf, size)
}

func (a *authSHA1V4) getRandDataLength(size int) int {
	if size > 1200 {
		return 0
	}
	if size > 400 {
		return rand.Intn(256)
	}
	return rand.Intn(512)
}
