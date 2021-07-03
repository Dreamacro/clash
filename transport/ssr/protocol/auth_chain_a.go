package protocol

import (
	"bytes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rc4"
	"encoding/base64"
	"encoding/binary"
	"net"
	"strconv"
	"strings"

	"github.com/Dreamacro/clash/common/pool"
	"github.com/Dreamacro/clash/log"
	"github.com/Dreamacro/clash/transport/ssr/tools"

	"github.com/Dreamacro/go-shadowsocks2/core"
)

func init() {
	register("auth_chain_a", newAuthChainA, 4)
}

type randDataLengthMethod func(int, []byte, *tools.XorShift128Plus) int

type authChainA struct {
	*Base
	*authData
	*userData
	iv             []byte
	salt           string
	hasSentHeader  bool
	rawTrans       bool
	lastClientHash []byte
	lastServerHash []byte
	encrypter      cipher.Stream
	decrypter      cipher.Stream
	randomClient   tools.XorShift128Plus
	randomServer   tools.XorShift128Plus
	randDataLength randDataLengthMethod
	packID         uint32
	recvID         uint32
}

func newAuthChainA(b *Base) Protocol {
	a := &authChainA{
		Base:     b,
		authData: &authData{},
		userData: &userData{},
		salt:     "auth_chain_a",
	}
	a.initUserData()
	return a
}

func (a *authChainA) initUserData() {
	params := strings.Split(a.Param, ":")
	if len(params) > 1 {
		if userID, err := strconv.ParseUint(params[0], 10, 32); err == nil {
			binary.LittleEndian.PutUint32(a.userID[:], uint32(userID))
			a.userKey = []byte(params[1])
		} else {
			log.Warnln("Wrong protocol-param for %s, only digits are expected before ':'", a.salt)
		}
	}
	if len(a.userKey) == 0 {
		a.userKey = a.Key
		rand.Read(a.userID[:])
	}
}

func (a *authChainA) StreamConn(c net.Conn, iv []byte) net.Conn {
	p := &authChainA{
		Base:     a.Base,
		authData: a.next(),
		userData: a.userData,
		salt:     a.salt,
		packID:   1,
		recvID:   1,
	}
	p.iv = iv
	p.randDataLength = p.getRandLength
	return &Conn{Conn: c, Protocol: p}
}

func (a *authChainA) PacketConn(c net.PacketConn) net.PacketConn {
	p := &authChainA{
		Base:     a.Base,
		salt:     a.salt,
		userData: a.userData,
	}
	return &PacketConn{PacketConn: c, Protocol: p}
}

func (a *authChainA) Decode(dst, src *bytes.Buffer) error {
	if a.rawTrans {
		dst.ReadFrom(src)
		return nil
	}
	for src.Len() > 4 {
		macKey := pool.Get(len(a.userKey) + 4)
		defer pool.Put(macKey)
		copy(macKey, a.userKey)
		binary.LittleEndian.PutUint32(macKey[len(a.userKey):], a.recvID)

		dataLength := int(binary.LittleEndian.Uint16(src.Bytes()[:2]) ^ binary.LittleEndian.Uint16(a.lastServerHash[14:16]))
		randDataLength := a.randDataLength(dataLength, a.lastServerHash, &a.randomServer)
		length := dataLength + randDataLength

		if length >= 4096 {
			a.rawTrans = true
			src.Reset()
			return errAuthChainLengthError
		}

		if 4+length > src.Len() {
			break
		}

		serverHash := tools.HmacMD5(macKey, src.Bytes()[:length+2])
		if !bytes.Equal(serverHash[:2], src.Bytes()[length+2:length+4]) {
			a.rawTrans = true
			src.Reset()
			return errAuthChainChksumError
		}
		a.lastServerHash = serverHash

		pos := 2
		if dataLength > 0 && randDataLength > 0 {
			pos += getRandStartPos(randDataLength, &a.randomServer)
		}
		wantedData := src.Bytes()[pos : pos+dataLength]
		a.decrypter.XORKeyStream(wantedData, wantedData)
		if a.recvID == 1 {
			dst.Write(wantedData[2:])
		} else {
			dst.Write(wantedData)
		}
		a.recvID++
		src.Next(length + 4)
	}
	return nil
}

func (a *authChainA) Encode(buf *bytes.Buffer, b []byte) error {
	if !a.hasSentHeader {
		dataLength := getDataLength(b)
		a.packAuthData(buf, b[:dataLength])
		b = b[dataLength:]
		a.hasSentHeader = true
	}
	for len(b) > 2800 {
		a.packData(buf, b[:2800])
		b = b[2800:]
	}
	if len(b) > 0 {
		a.packData(buf, b)
	}
	return nil
}

func (a *authChainA) DecodePacket(b []byte) ([]byte, error) {
	if len(b) < 9 {
		return nil, errAuthChainLengthError
	}
	if !bytes.Equal(tools.HmacMD5(a.userKey, b[:len(b)-1])[:1], b[len(b)-1:]) {
		return nil, errAuthChainChksumError
	}
	md5Data := tools.HmacMD5(a.Key, b[len(b)-8:len(b)-1])

	randDataLength := udpGetRandLength(md5Data, &a.randomServer)

	key := core.Kdf(base64.StdEncoding.EncodeToString(a.userKey)+base64.StdEncoding.EncodeToString(md5Data), 16)
	rc4Cipher, err := rc4.NewCipher(key)
	if err != nil {
		return nil, err
	}
	wantedData := b[:len(b)-8-randDataLength]
	rc4Cipher.XORKeyStream(wantedData, wantedData)
	return wantedData, nil
}

func (a *authChainA) EncodePacket(buf *bytes.Buffer, b []byte) error {
	authData := pool.Get(3)
	defer pool.Put(authData)
	rand.Read(authData)

	md5Data := tools.HmacMD5(a.Key, authData)

	randDataLength := udpGetRandLength(md5Data, &a.randomClient)

	key := core.Kdf(base64.StdEncoding.EncodeToString(a.userKey)+base64.StdEncoding.EncodeToString(md5Data), 16)
	rc4Cipher, err := rc4.NewCipher(key)
	if err != nil {
		return err
	}
	rc4Cipher.XORKeyStream(b, b)

	buf.Write(b)
	tools.AppendRandBytes(buf, randDataLength)
	buf.Write(authData)
	binary.Write(buf, binary.LittleEndian, binary.LittleEndian.Uint32(a.userID[:])^binary.LittleEndian.Uint32(md5Data[:4]))
	buf.Write(tools.HmacMD5(a.userKey, buf.Bytes())[:1])
	return nil
}

func (a *authChainA) packAuthData(poolBuf *bytes.Buffer, data []byte) {
	/*
		dataLength := len(data)
		12:	checkHead(4) and hmac of checkHead(8)
		4:	uint32 LittleEndian uid (uid = userID ^ last client hash)
		16:	encrypted data of authdata(12), uint16 LittleEndian overhead(2) and uint16 LittleEndian number zero(2)
		4:	last server hash(4)
		packedAuthDataLength := 12 + 4 + 16 + 4 + dataLength
	*/

	macKey := pool.Get(len(a.iv) + len(a.Key))
	defer pool.Put(macKey)
	copy(macKey, a.iv)
	copy(macKey[len(a.iv):], a.Key)

	// check head
	tools.AppendRandBytes(poolBuf, 4)
	a.lastClientHash = tools.HmacMD5(macKey, poolBuf.Bytes())
	a.initRC4Cipher()
	poolBuf.Write(a.lastClientHash[:8])
	// uid
	binary.Write(poolBuf, binary.LittleEndian, binary.LittleEndian.Uint32(a.userID[:])^binary.LittleEndian.Uint32(a.lastClientHash[8:12]))
	// encrypted data
	err := a.putEncryptedData(poolBuf, a.userKey, [2]int{a.Overhead, 0}, a.salt)
	if err != nil {
		poolBuf.Reset()
		return
	}
	// last server hash
	a.lastServerHash = tools.HmacMD5(a.userKey, poolBuf.Bytes()[12:])
	poolBuf.Write(a.lastServerHash[:4])
	// packed data
	a.packData(poolBuf, data)
}

func (a *authChainA) packData(poolBuf *bytes.Buffer, data []byte) {
	a.encrypter.XORKeyStream(data, data)

	macKey := pool.Get(len(a.userKey) + 4)
	defer pool.Put(macKey)
	copy(macKey, a.userKey)
	binary.LittleEndian.PutUint32(macKey[len(a.userKey):], a.packID)
	a.packID++

	length := uint16(len(data)) ^ binary.LittleEndian.Uint16(a.lastClientHash[14:16])

	originalLength := poolBuf.Len()
	binary.Write(poolBuf, binary.LittleEndian, length)
	a.putMixedRandDataAndData(poolBuf, data)
	a.lastClientHash = tools.HmacMD5(macKey, poolBuf.Bytes()[originalLength:])
	poolBuf.Write(a.lastClientHash[:2])
}

func (a *authChainA) putMixedRandDataAndData(poolBuf *bytes.Buffer, data []byte) {
	randDataLength := a.randDataLength(len(data), a.lastClientHash, &a.randomClient)
	if len(data) == 0 {
		tools.AppendRandBytes(poolBuf, randDataLength)
		return
	}
	if randDataLength > 0 {
		startPos := getRandStartPos(randDataLength, &a.randomClient)
		tools.AppendRandBytes(poolBuf, startPos)
		poolBuf.Write(data)
		tools.AppendRandBytes(poolBuf, randDataLength-startPos)
		return
	}
	poolBuf.Write(data)
}

func getRandStartPos(length int, random *tools.XorShift128Plus) int {
	if length == 0 {
		return 0
	}
	return int(random.Next()%8589934609) % length
}

func (a *authChainA) getRandLength(length int, lastHash []byte, random *tools.XorShift128Plus) int {
	if length > 1440 {
		return 0
	}
	random.InitFromBinAndLength(lastHash, length)
	if length > 1300 {
		return int(random.Next() % 31)
	}
	if length > 900 {
		return int(random.Next() % 127)
	}
	if length > 400 {
		return int(random.Next() % 521)
	}
	return int(random.Next() % 1021)
}

func (a *authChainA) initRC4Cipher() {
	key := core.Kdf(base64.StdEncoding.EncodeToString(a.userKey)+base64.StdEncoding.EncodeToString(a.lastClientHash), 16)
	a.encrypter, _ = rc4.NewCipher(key)
	a.decrypter, _ = rc4.NewCipher(key)
}

func udpGetRandLength(lastHash []byte, random *tools.XorShift128Plus) int {
	random.InitFromBin(lastHash)
	return int(random.Next() % 127)
}
