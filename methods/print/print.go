package main

import (
	"log"
	"os"

	yozoraMethods "github.com/ShashankRaoCoding/yozora/methods"
)

var printMethods = map[string]func([]string){}

func init() {
	yozoraMethods.Methods["print"] = print
}

func print(args []string) {
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
