package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jumpcrypto/crosschain/config"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("usage: ./main <secret-reference>")
		fmt.Println()
		fmt.Println("example: ./main gsm:myproject,mysecret")
		return
	}
	sec, err := config.GetSecret(os.Args[1])
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(sec)
}
