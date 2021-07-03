package tools

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
)

const HmacSHA1Len = 10

func HmacMD5(key, data []byte) []byte {
	hmacMD5 := hmac.New(md5.New, key)
	hmacMD5.Write(data)
	return hmacMD5.Sum(nil)
}

func HmacSHA1(key, data []byte) []byte {
	hmacSHA1 := hmac.New(sha1.New, key)
	hmacSHA1.Write(data)
	return hmacSHA1.Sum(nil)
}

func MD5Sum(b []byte) []byte {
	h := md5.New()
	h.Write(b)
	return h.Sum(nil)
}

func SHA1Sum(b []byte) []byte {
	h := sha1.New()
	h.Write(b)
	return h.Sum(nil)
}
