package main

import (
	"github.com/mildred/assrt"
	"testing"
)

func TestDateAdd(t *testing.T) {
	assert := assrt.NewAssert(t)
	res, err := dateadd("25/12/2016", 1)
	assert.Nil(err)
	assert.Equal("26/12/2016", res)
}
