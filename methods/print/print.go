package print

import (
	"log"
	"os"

	methods "github.com/ShashankRaoCoding/yozora/methods"
)

var Methods = map[string]func([]string){}

func init() {
	methods.Methods["print"] = print
}

func print(args []string) {
	if len(args) < 1 {
		log.Println("Error: no method specified")
		os.Exit(1)
	}

	m, args := args[0], args[1:]
	f, o := Methods[m]
	if o {
		f(args)
	} else {
		log.Println("Error: unconfigured method")
		os.Exit(1)
	}
}
