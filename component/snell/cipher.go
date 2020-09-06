package snell

import (
	"crypto/aes"
	"crypto/cipher"

	"github.com/Dreamacro/go-shadowsocks2/shadowaead"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

type snellCipher struct {
	psk      []byte
	keySize  int
	makeAEAD func(key []byte) (cipher.AEAD, error)
}

func (sc *snellCipher) KeySize() int  { return sc.keySize }
func (sc *snellCipher) SaltSize() int { return 16 }
func (sc *snellCipher) Encrypter(salt []byte) (cipher.AEAD, error) {
	return sc.makeAEAD(snellKDF(sc.psk, salt, sc.KeySize()))
}
func (sc *snellCipher) Decrypter(salt []byte) (cipher.AEAD, error) {
	return sc.makeAEAD(snellKDF(sc.psk, salt, sc.KeySize()))
}

func snellKDF(psk, salt []byte, keySize int) []byte {
	// snell use a special kdf function
	return argon2.IDKey(psk, salt, 3, 8, 1, 32)[:keySize]
}

func aesGCM(key []byte) (cipher.AEAD, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(blk)
}

func NewAES128GCM(psk []byte) shadowaead.Cipher {
	return &snellCipher{
		psk:      psk,
		keySize:  16,
		makeAEAD: aesGCM,
	}
}

func NewChacha20Poly1305(psk []byte) shadowaead.Cipher {
	return &snellCipher{
		psk:      psk,
		keySize:  32,
		makeAEAD: chacha20poly1305.New,
	}
}
