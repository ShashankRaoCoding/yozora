package main

import (
	"github.com/ShashankRaoCoding/yozora/methods/print"
)

func init() {
	print.Methods["image"] = image
}

func image(_ []string) {
	// write sixel or kitty or ascii

	if sixelCompatible() {
		renderSixel()
	} else if kittyCompatible() {
		renderKitty()
	} else {
		renderASCII()
	}
}
