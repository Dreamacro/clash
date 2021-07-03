package protocol

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
	"net"
	"strconv"
	"strings"

	"github.com/Dreamacro/clash/common/pool"
	"github.com/Dreamacro/clash/log"
	"github.com/Dreamacro/clash/transport/ssr/tools"
)

type hmacMethod func(key, data []byte) []byte
type hashDigestMethod func([]byte) []byte

func init() {
	register("auth_aes128_sha1", newAuthAES128SHA1, 9)
}

type authAES128Function struct {
	salt       string
	hmac       hmacMethod
	hashDigest hashDigestMethod
}

type authAES128 struct {
	*Base
	*authData
	*authAES128Function
	*userData
	iv            []byte
	hasSentHeader bool
	rawTrans      bool
	packID        uint32
	recvID        uint32
}

func newAuthAES128SHA1(b *Base) Protocol {
	a := &authAES128{
		Base:               b,
		authData:           &authData{},
		authAES128Function: &authAES128Function{salt: "auth_aes128_sha1", hmac: tools.HmacSHA1, hashDigest: tools.SHA1Sum},
		userData:           &userData{},
	}
	a.initUserData()
	return a
}

func (a *authAES128) initUserData() {
	params := strings.Split(a.Param, ":")
	if len(params) > 1 {
		if userID, err := strconv.ParseUint(params[0], 10, 32); err == nil {
			binary.LittleEndian.PutUint32(a.userID[:], uint32(userID))
			a.userKey = a.hashDigest([]byte(params[1]))
		} else {
			log.Warnln("Wrong protocol-param for %s, only digits are expected before ':'", a.salt)
		}
	}
	if len(a.userKey) == 0 {
		a.userKey = a.Key
		rand.Read(a.userID[:])
	}
}

func (a *authAES128) StreamConn(c net.Conn, iv []byte) net.Conn {
	p := &authAES128{
		Base:               a.Base,
		authData:           a.next(),
		authAES128Function: a.authAES128Function,
		userData:           a.userData,
		packID:             1,
		recvID:             1,
	}
	p.iv = iv
	return &Conn{Conn: c, Protocol: p}
}

func (a *authAES128) PacketConn(c net.PacketConn) net.PacketConn {
	p := &authAES128{
		Base:               a.Base,
		authAES128Function: a.authAES128Function,
		userData:           a.userData,
	}
	return &PacketConn{PacketConn: c, Protocol: p}
}

func (a *authAES128) Decode(dst, src *bytes.Buffer) error {
	if a.rawTrans {
		dst.ReadFrom(src)
		return nil
	}
	for src.Len() > 4 {
		macKey := pool.Get(len(a.userKey) + 4)
		defer pool.Put(macKey)
		copy(macKey, a.userKey)
		binary.LittleEndian.PutUint32(macKey[len(a.userKey):], a.recvID)
		if !bytes.Equal(a.hmac(macKey, src.Bytes()[:2])[:2], src.Bytes()[2:4]) {
			src.Reset()
			return errAuthAES128MACError
		}

		length := int(binary.LittleEndian.Uint16(src.Bytes()[:2]))
		if length >= 8192 || length < 7 {
			a.rawTrans = true
			src.Reset()
			return errAuthAES128LengthError
		}
		if length > src.Len() {
			break
		}

		if !bytes.Equal(a.hmac(macKey, src.Bytes()[:length-4])[:4], src.Bytes()[length-4:length]) {
			a.rawTrans = true
			src.Reset()
			return errAuthAES128ChksumError
		}

		a.recvID++

		pos := int(src.Bytes()[4])
		if pos < 255 {
			pos += 4
		} else {
			pos = int(binary.LittleEndian.Uint16(src.Bytes()[5:7])) + 4
		}
		dst.Write(src.Bytes()[pos : length-4])
		src.Next(length)
	}
	return nil
}

func (a *authAES128) Encode(buf *bytes.Buffer, b []byte) error {
	fullDataLength := len(b)
	if !a.hasSentHeader {
		dataLength := getDataLength(b)
		a.packAuthData(buf, b[:dataLength])
		b = b[dataLength:]
		a.hasSentHeader = true
	}
	for len(b) > 8100 {
		a.packData(buf, b[:8100], fullDataLength)
		b = b[8100:]
	}
	if len(b) > 0 {
		a.packData(buf, b, fullDataLength)
	}
	return nil
}

func (a *authAES128) DecodePacket(b []byte) ([]byte, error) {
	if !bytes.Equal(a.hmac(a.Key, b[:len(b)-4])[:4], b[len(b)-4:]) {
		return nil, errAuthAES128ChksumError
	}
	return b[:len(b)-4], nil
}

func (a *authAES128) EncodePacket(buf *bytes.Buffer, b []byte) error {
	buf.Write(b)
	buf.Write(a.userID[:])
	buf.Write(a.hmac(a.userKey, buf.Bytes())[:4])
	return nil
}

func (a *authAES128) packData(poolBuf *bytes.Buffer, data []byte, fullDataLength int) {
	dataLength := len(data)
	randDataLength := a.getRandDataLengthForPackData(dataLength, fullDataLength)
	/*
		2:	uint16 LittleEndian packedDataLength
		2:	hmac of packedDataLength
		3:	maxRandDataLengthPrefix (min:1)
		4:	hmac of packedData except the last 4 bytes
	*/
	packedDataLength := 2 + 2 + 3 + randDataLength + dataLength + 4
	if randDataLength < 128 {
		packedDataLength -= 2
	}

	macKey := pool.Get(len(a.userKey) + 4)
	defer pool.Put(macKey)
	copy(macKey, a.userKey)
	binary.LittleEndian.PutUint32(macKey[len(a.userKey):], a.packID)
	a.packID++

	binary.Write(poolBuf, binary.LittleEndian, uint16(packedDataLength))
	poolBuf.Write(a.hmac(macKey, poolBuf.Bytes()[poolBuf.Len()-2:])[:2])
	a.packRandData(poolBuf, randDataLength)
	poolBuf.Write(data)
	poolBuf.Write(a.hmac(macKey, poolBuf.Bytes()[poolBuf.Len()-packedDataLength+4:])[:4])
}

func trapezoidRandom(max int, d float64) int {
	base := rand.Float64()
	if d-0 > 1e-6 {
		a := 1 - d
		base = (math.Sqrt(a*a+4*d*base) - a) / (2 * d)
	}
	return int(base * float64(max))
}

func (a *authAES128) getRandDataLengthForPackData(dataLength, fullDataLength int) int {
	if fullDataLength >= 32*1024-a.Overhead {
		return 0
	}
	// 1460: tcp_mss
	revLength := 1460 - dataLength - 9
	if revLength == 0 {
		return 0
	}
	if revLength < 0 {
		if revLength > -1460 {
			return trapezoidRandom(revLength+1460, -0.3)
		}
		return rand.Intn(32)
	}
	if dataLength > 900 {
		return rand.Intn(revLength)
	}
	return trapezoidRandom(revLength, -0.3)
}

func (a *authAES128) packAuthData(poolBuf *bytes.Buffer, data []byte) {
	if len(data) == 0 {
		return
	}
	dataLength := len(data)
	randDataLength := a.getRandDataLengthForPackAuthData(dataLength)
	/*
		7:	checkHead(1) and hmac of checkHead(6)
		4:	userID
		16:	encrypted data of authdata(12), uint16 BigEndian packedDataLength(2) and uint16 BigEndian randDataLength(2)
		4:	hmac of userID and encrypted data
		4:	hmac of packedAuthData except the last 4 bytes
	*/
	packedAuthDataLength := 7 + 4 + 16 + 4 + randDataLength + dataLength + 4

	macKey := pool.Get(len(a.iv) + len(a.Key))
	defer pool.Put(macKey)
	copy(macKey, a.iv)
	copy(macKey[len(a.iv):], a.Key)

	poolBuf.WriteByte(byte(rand.Intn(256)))
	poolBuf.Write(a.hmac(macKey, poolBuf.Bytes())[:6])
	poolBuf.Write(a.userID[:])
	err := a.authData.putEncryptedData(poolBuf, a.userKey, [2]int{packedAuthDataLength, randDataLength}, a.salt)
	if err != nil {
		poolBuf.Reset()
		return
	}
	poolBuf.Write(a.hmac(macKey, poolBuf.Bytes()[7:])[:4])
	tools.AppendRandBytes(poolBuf, randDataLength)
	poolBuf.Write(data)
	poolBuf.Write(a.hmac(a.userKey, poolBuf.Bytes())[:4])
}

func (a *authAES128) getRandDataLengthForPackAuthData(size int) int {
	if size > 400 {
		return rand.Intn(512)
	}
	return rand.Intn(1024)
}

func (a *authAES128) packRandData(poolBuf *bytes.Buffer, size int) {
	if size < 128 {
		poolBuf.WriteByte(byte(size + 1))
		tools.AppendRandBytes(poolBuf, size)
		return
	}
	poolBuf.WriteByte(255)
	binary.Write(poolBuf, binary.LittleEndian, uint16(size+3))
	tools.AppendRandBytes(poolBuf, size)
}
