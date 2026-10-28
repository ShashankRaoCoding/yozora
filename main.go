package main

import (
	"log"
	"os"

	"github.com/ShashankRaoCoding/yozora/methods"
)

func main() {
	args := os.Args[1:]
	if len(args) < 1 {
		log.Println("Error: no method specified")
		os.Exit(1)
	}

	m, args := args[0], args[1:]
	f, o := methods.Methods[m]
	if o {
		f(args)
	} else {
		log.Println("Error: unconfigured method")
		os.Exit(1)
	}
}
