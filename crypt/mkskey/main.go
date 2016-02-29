package main

import (
	"fmt"
	"github.com/jbenet/go-base58"
	"github.com/mildred/sogiboard/crypt"
	"os"
)

func main() {
	sk, err := crypt.NewSecretKey()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	sktext := base58.Encode(sk)
	fmt.Printf("SECRET_KEY=\"%s\"\n", sktext)
}
