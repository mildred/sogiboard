package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
)

const BlockSize = aes.BlockSize

func Encrypt(key []byte, data []byte) ([]byte, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	iv := make([]byte, BlockSize)
	n, err := rand.Read(iv)
	if n != BlockSize {
		panic("Could not read enough random bytes")
	} else if err != nil {
		return nil, err
	}
	enc := cipher.NewCFBEncrypter(blk, iv)
	res := make([]byte, len(data))
	enc.XORKeyStream(res, data)
	return append(iv, res...), nil
}

func Decrypt(key []byte, data []byte) ([]byte, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	iv := data[:BlockSize]
	data = data[BlockSize:]
	enc := cipher.NewCFBDecrypter(blk, iv)
	res := make([]byte, len(data))
	enc.XORKeyStream(res, data)
	return res, nil
}

func NewSecretKey() ([]byte, error) {
	skey := make([]byte, BlockSize)
	n, err := rand.Read(skey)
	if n != BlockSize {
		panic("Could not read enough random bytes")
	} else if err != nil {
		return nil, err
	}
	return skey, nil
}
