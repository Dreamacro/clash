package protocol

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/Dreamacro/clash/common/pool"
	"github.com/Dreamacro/clash/component/ssr/tools"
	"github.com/Dreamacro/go-shadowsocks2/core"
)

type authAES128 struct {
	*Base
	*recvInfo
	*authData
	hasSentHeader bool
	packID        uint32
	userKey       []byte
	uid           [4]byte
	salt          string
	hmac          hmacMethod
	hashDigest    hashDigestMethod
}

func init() {
	register("auth_aes128_md5", newAuthAES128MD5)
}

func newAuthAES128MD5(b *Base) Protocol {
	return &authAES128{
		Base:       b,
		authData:   &authData{},
		salt:       "auth_aes128_md5",
		hmac:       tools.HmacMD5,
		hashDigest: tools.MD5Sum,
	}
}

func (a *authAES128) initForConn(iv []byte) Protocol {
	return &authAES128{
		Base: &Base{
			IV:       iv,
			Key:      a.Key,
			TCPMss:   a.TCPMss,
			Overhead: a.Overhead,
			Param:    a.Param,
		},
		recvInfo:   &recvInfo{recvID: 1, buffer: new(bytes.Buffer)},
		authData:   a.authData,
		packID:     1,
		salt:       a.salt,
		hmac:       a.hmac,
		hashDigest: a.hashDigest,
	}
}

func (a *authAES128) GetProtocolOverhead() int {
	return 9
}

func (a *authAES128) SetOverhead(overhead int) {
	a.Overhead = overhead
}

func (a *authAES128) Decode(b []byte) ([]byte, int, error) {
	a.buffer.Reset()
	bSize := len(b)
	readSize := 0
	key := pool.Get(len(a.userKey) + 4)
	defer pool.Put(key)
	copy(key, a.userKey)
	for bSize > 4 {
		binary.LittleEndian.PutUint32(key[len(key)-4:], a.recvID)

		h := a.hmac(key, b[:2])
		if !bytes.Equal(h[:2], b[2:4]) {
			return nil, 0, errAuthAES128IncorrectMAC
		}

		length := int(binary.LittleEndian.Uint16(b[:2]))
		if length >= 8192 || length < 8 {
			return nil, 0, errAuthAES128DataLengthError
		}
		if length > bSize {
			break
		}

		h = a.hmac(key, b[:length-4])
		if !bytes.Equal(h[:4], b[length-4:length]) {
			return nil, 0, errAuthAES128IncorrectChecksum
		}

		a.recvID++
		pos := int(b[4])
		if pos < 255 {
			pos += 4
		} else {
			pos = int(binary.LittleEndian.Uint16(b[5:7])) + 4
		}

		if pos > length-4 {
			return nil, 0, errAuthAES128PositionTooLarge
		}
		a.buffer.Write(b[pos : length-4])
		b = b[length:]
		bSize -= length
		readSize += length
	}
	return a.buffer.Bytes(), readSize, nil
}

func (a *authAES128) Encode(b []byte) ([]byte, error) {
	a.buffer.Reset()
	bSize := len(b)
	offset := 0
	if bSize > 0 && !a.hasSentHeader {
		authSize := bSize
		if authSize > 1200 {
			authSize = 1200
		}
		a.hasSentHeader = true
		a.buffer.Write(a.packAuthData(b[:authSize]))
		bSize -= authSize
		offset += authSize
	}
	const blockSize = 4096
	for bSize > blockSize {
		packSize, randSize := a.packedDataSize(b[offset : offset+blockSize])
		pack := pool.Get(packSize)
		a.packData(b[offset:offset+blockSize], pack, randSize)
		a.buffer.Write(pack)
		pool.Put(pack)
		bSize -= blockSize
		offset += blockSize
	}
	if bSize > 0 {
		packSize, randSize := a.packedDataSize(b[offset:])
		pack := pool.Get(packSize)
		a.packData(b[offset:], pack, randSize)
		a.buffer.Write(pack)
		pool.Put(pack)
	}
	return a.buffer.Bytes(), nil
}

func (a *authAES128) DecodePacket(b []byte) ([]byte, int, error) {
	bSize := len(b)
	h := a.hmac(a.Key, b[:bSize-4])
	if !bytes.Equal(h[:4], b[bSize-4:]) {
		return nil, 0, errAuthAES128IncorrectMAC
	}
	return b[:bSize-4], bSize - 4, nil
}

func (a *authAES128) EncodePacket(b []byte) ([]byte, error) {
	a.initUserKeyAndID()

	var buf bytes.Buffer
	buf.Write(b)
	buf.Write(a.uid[:])
	h := a.hmac(a.userKey, buf.Bytes())
	buf.Write(h[:4])
	return buf.Bytes(), nil
}

func (a *authAES128) initUserKeyAndID() {
	if a.userKey == nil {
		params := strings.Split(a.Param, ":")
		if len(params) >= 2 {
			if userID, err := strconv.ParseUint(params[0], 10, 32); err == nil {
				binary.LittleEndian.PutUint32(a.uid[:], uint32(userID))
				a.userKey = a.hashDigest([]byte(params[1]))
			}
		}

		if a.userKey == nil {
			rand.Read(a.uid[:])
			a.userKey = make([]byte, len(a.Key))
			copy(a.userKey, a.Key)
		}
	}
}

func (a *authAES128) packedDataSize(data []byte) (packSize, randSize int) {
	dataSize := len(data)
	randSize = 1
	if dataSize <= 1200 {
		if a.packID > 4 {
			randSize += rand.Intn(32)
		} else {
			if dataSize > 900 {
				randSize += rand.Intn(128)
			} else {
				randSize += rand.Intn(512)
			}
		}
	}
	packSize = randSize + dataSize + 8
	return
}

func (a *authAES128) packData(data, ret []byte, randSize int) {
	dataSize := len(data)
	retSize := len(ret)
	// 0~1, ret_size
	binary.LittleEndian.PutUint16(ret[0:], uint16(retSize&0xFFFF))
	// 2~3, hmac
	key := pool.Get(len(a.userKey) + 4)
	defer pool.Put(key)
	copy(key, a.userKey)
	binary.LittleEndian.PutUint32(key[len(key)-4:], a.packID)
	h := a.hmac(key, ret[:2])
	copy(ret[2:4], h[:2])
	// 4~rand_size+4, rand number
	rand.Read(ret[4 : 4+randSize])
	// 4, rand_size
	if randSize < 128 {
		ret[4] = byte(randSize & 0xFF)
	} else {
		// 4, magic number 0xFF
		ret[4] = 0xFF
		// 5~6, rand_size
		binary.LittleEndian.PutUint16(ret[5:], uint16(randSize&0xFFFF))
	}
	// rand_size+4~ret_size-4, data
	if dataSize > 0 {
		copy(ret[randSize+4:], data)
	}
	a.packID++
	h = a.hmac(key, ret[:retSize-4])
	copy(ret[retSize-4:], h[:4])
}

func (a *authAES128) packAuthData(data []byte) (ret []byte) {
	dataSize := len(data)
	var randSize int

	if dataSize > 400 {
		randSize = rand.Intn(512)
	} else {
		randSize = rand.Intn(1024)
	}

	dataOffset := randSize + 16 + 4 + 4 + 7
	retSize := dataOffset + dataSize + 4
	ret = make([]byte, retSize)
	encrypt := make([]byte, 24)
	key := make([]byte, len(a.IV)+len(a.Key))
	copy(key, a.IV)
	copy(key[len(a.IV):], a.Key)

	rand.Read(ret[dataOffset-randSize:])
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.connectionID++
	if a.connectionID > 0xFF000000 {
		a.clientID = nil
	}
	if len(a.clientID) == 0 {
		a.clientID = make([]byte, 8)
		rand.Read(a.clientID)
		b := make([]byte, 4)
		rand.Read(b)
		a.connectionID = binary.LittleEndian.Uint32(b) & 0xFFFFFF
	}
	copy(encrypt[4:], a.clientID)
	binary.LittleEndian.PutUint32(encrypt[8:], a.connectionID)

	now := time.Now().Unix()
	binary.LittleEndian.PutUint32(encrypt[:4], uint32(now))

	binary.LittleEndian.PutUint16(encrypt[12:], uint16(retSize&0xFFFF))
	binary.LittleEndian.PutUint16(encrypt[14:], uint16(randSize&0xFFFF))

	a.initUserKeyAndID()

	aesCipherKey := core.Kdf(base64.StdEncoding.EncodeToString(a.userKey)+a.salt, 16)
	block, err := aes.NewCipher(aesCipherKey)
	if err != nil {
		return nil
	}
	encryptData := make([]byte, 16)
	iv := make([]byte, aes.BlockSize)
	cbc := cipher.NewCBCEncrypter(block, iv)
	cbc.CryptBlocks(encryptData, encrypt[:16])
	copy(encrypt[:4], a.uid[:])
	copy(encrypt[4:4+16], encryptData)

	h := a.hmac(key, encrypt[:20])
	copy(encrypt[20:], h[:4])

	rand.Read(ret[:1])
	h = a.hmac(key, ret[:1])
	copy(ret[1:], h[:7-1])

	copy(ret[7:], encrypt)
	copy(ret[dataOffset:], data)

	h = a.hmac(a.userKey, ret[:retSize-4])
	copy(ret[retSize-4:], h[:4])

	return
}
