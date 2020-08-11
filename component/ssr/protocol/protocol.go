package protocol

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"
)

var (
	errAuthAES128IncorrectMAC      = errors.New("auth_aes128_* post decrypt incorrect mac")
	errAuthAES128DataLengthError   = errors.New("auth_aes128_* post decrypt length mismatch")
	errAuthAES128IncorrectChecksum = errors.New("auth_aes128_* post decrypt incorrect checksum")
	errAuthAES128PositionTooLarge  = errors.New("auth_aes128_* post decrypt position is too large")
	errAuthSHA1v4CRC32Error        = errors.New("auth_sha1_v4 post decrypt data crc32 error")
	errAuthSHA1v4DataLengthError   = errors.New("auth_sha1_v4 post decrypt data length error")
	errAuthSHA1v4IncorrectChecksum = errors.New("auth_sha1_v4 post decrypt incorrect checksum")
	errAuthChainDataLengthError    = errors.New("auth_chain_* post decrypt length mismatch")
	errAuthChainHMACError          = errors.New("auth_chain_* post decrypt hmac error")
)

type authData struct {
	clientID     []byte
	connectionID uint32
	mutex        sync.Mutex
}

type recvInfo struct {
	recvID uint32
	buffer *bytes.Buffer
}

type hmacMethod func(key []byte, data []byte) []byte
type hashDigestMethod func(data []byte) []byte
type rndMethod func(dataSize int, random *shift128PlusContext, lastHash []byte, dataSizeList, dataSizeList2 []int, overhead int) int

// Protocol provides methods for decoding, encoding and iv setting
type Protocol interface {
	initForConn(iv []byte) Protocol
	GetProtocolOverhead() int
	SetOverhead(int)
	Decode([]byte) ([]byte, int, error)
	Encode([]byte) ([]byte, error)
	DecodePacket([]byte) ([]byte, int, error)
	EncodePacket([]byte) ([]byte, error)
}

type protocolCreator func(b *Base) Protocol

var protocolList = make(map[string]protocolCreator)

func register(name string, c protocolCreator) {
	protocolList[name] = c
}

// PickProtocol returns a protocol of the given name
func PickProtocol(name string, b *Base) (Protocol, error) {
	if protocolCreator, ok := protocolList[strings.ToLower(name)]; ok {
		return protocolCreator(b), nil
	}
	return nil, fmt.Errorf("Protocol %s not supported", name)
}
