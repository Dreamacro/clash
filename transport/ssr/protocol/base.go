package protocol

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"math/rand"
	"sync"
	"time"

	"github.com/Dreamacro/clash/common/pool"
	"github.com/Dreamacro/clash/log"

	"github.com/Dreamacro/go-shadowsocks2/core"
)

type Base struct {
	Key      []byte
	Overhead int
	Param    string
}

type userData struct {
	userKey []byte
	userID  [4]byte
}

type authData struct {
	clientID     [4]byte
	connectionID uint32
	mutex        sync.Mutex
}

func (a *authData) next() *authData {
	r := &authData{}
	a.mutex.Lock()
	defer a.mutex.Unlock()
	if a.connectionID > 0xff000000 || a.connectionID == 0 {
		rand.Read(a.clientID[:])
		a.connectionID = rand.Uint32() & 0xffffff
	}
	a.connectionID++
	copy(r.clientID[:], a.clientID[:])
	r.connectionID = a.connectionID
	return r
}

func (a *authData) putAuthData(buf *bytes.Buffer) {
	binary.Write(buf, binary.LittleEndian, uint32(time.Now().Unix()))
	buf.Write(a.clientID[:])
	binary.Write(buf, binary.LittleEndian, a.connectionID)
}

func (a *authData) putEncryptedData(b *bytes.Buffer, userKey []byte, paddings [2]int, salt string) error {
	encrypt := pool.Get(16)
	defer pool.Put(encrypt)
	binary.LittleEndian.PutUint32(encrypt, uint32(time.Now().Unix()))
	copy(encrypt[4:], a.clientID[:])
	binary.LittleEndian.PutUint32(encrypt[8:], a.connectionID)
	binary.LittleEndian.PutUint16(encrypt[12:], uint16(paddings[0]))
	binary.LittleEndian.PutUint16(encrypt[14:], uint16(paddings[1]))

	cipherKey := core.Kdf(base64.StdEncoding.EncodeToString(userKey)+salt, 16)
	block, err := aes.NewCipher(cipherKey)
	if err != nil {
		log.Warnln("New cipher error: %s", err.Error())
		return err
	}
	iv := bytes.Repeat([]byte{0}, 16)
	cbcCipher := cipher.NewCBCEncrypter(block, iv)

	cbcCipher.CryptBlocks(encrypt, encrypt)

	b.Write(encrypt)
	return nil
}
