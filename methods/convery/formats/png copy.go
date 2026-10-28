package image

import (
	"github.com/ShashankRaoCoding/yozora/methods/print"
)

func init() {
	print.Methods[[2]string{"mrc", "png"}] = mrc2png
}

func mrc2png(_ []string) {
	// read and print to stdin and stdout
}
