package print

import (
	"log"
	"os"

	methods "github.com/ShashankRaoCoding/yozora/methods"
)

var Methods = map[string]func([]string){}

func init() {
	methods.Methods["convert"] = convert
}

func convert(args []string) {
	// flags -i input format -o output format
	f, o := Methods[[2]string{*inputFormat, *outputFormat}]
	if o {
		f(args)
	} else {
		log.Println("Unconfigured file type combination")
		os.Exit(1)
	}
}
