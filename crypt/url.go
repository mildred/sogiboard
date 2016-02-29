package crypt

import (
	"github.com/jbenet/go-base58"
	"net/url"
)

func DecryptUrl(skey []byte, u *url.URL) (*url.URL, error) {
	var res url.URL = *u
	data := base58.Decode(u.RawQuery)
	decrypted, err := Decrypt(skey, data)
	if err != nil {
		return nil, err
	}
	res.RawQuery = string(decrypted)
	return &res, nil
}

func EncryptUrl(skey []byte, u *url.URL) (*url.URL, error) {
	var res url.URL = *u
	encrypted, err := Encrypt(skey, []byte(u.RawQuery))
	if err != nil {
		return nil, err
	}
	res.RawQuery = base58.Encode(encrypted)
	return &res, nil
}

func EncryptQueryToBase58(skey []byte, qs url.Values) (string, error) {
	return EncryptRawQueryToBase58(skey, qs.Encode())
}

func EncryptRawQueryToBase58(skey []byte, query string) (string, error) {
	encrypted, err := Encrypt(skey, []byte(query))
	if err != nil {
		return "", err
	}
	return base58.Encode(encrypted), nil
}

func DecryptBase58RawQuery(skey []byte, enc string) (string, error) {
	data := base58.Decode(enc)
	decrypted, err := Decrypt(skey, data)
	if err != nil {
		return "", err
	}
	return string(decrypted), nil
}

func DecryptBase58Query(skey []byte, enc string) (url.Values, error) {
	data := base58.Decode(enc)
	decrypted, err := Decrypt(skey, data)
	if err != nil {
		return nil, err
	}
	res, err := url.ParseQuery(string(decrypted))
	if err != nil {
		return nil, err
	}
	return res, nil
}
