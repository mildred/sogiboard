package crypt

import (
	"github.com/mildred/assrt"
	"testing"
)

func TestCrypt(t *testing.T) {
	assert := assrt.NewAssert(t)
	skey, err := NewSecretKey()
	assert.Nil(err)

	res, err := Encrypt(skey, []byte("foobar"))
	assert.Nil(err)

	assert.NotEqual("foobar", res)

	orig, err := Decrypt(skey, res)
	assert.Nil(err)

	assert.Equal([]byte("foobar"), orig)
}
