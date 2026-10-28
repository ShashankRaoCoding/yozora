// Package convert implements `yozora convert -i <fmt> -o <fmt>`,
// reading from stdin and writing to stdout.
package convert

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/ShashankRaoCoding/yozora/methods"
)

// Methods maps {input format, output format} to a converter. Converters
// receive the remaining (non-flag) args and use stdin/stdout.
var Methods = map[[2]string]func([]string){}

func init() {
	methods.Methods["convert"] = convert
}

func convert(args []string) {
	fs := flag.NewFlagSet("convert", flag.ExitOnError)
	in := fs.String("i", "", "input format")
	out := fs.String("o", "", "output format")
	fs.Parse(args)

	if f, ok := Methods[[2]string{*in, *out}]; ok {
		f(fs.Args())
		return
	}
	var avail []string
	for k := range Methods {
		avail = append(avail, k[0]+"->"+k[1])
	}
	sort.Strings(avail)
	fmt.Fprintf(os.Stderr, "Error: unconfigured file type combination %q -> %q; available: %v\n", *in, *out, avail)
	os.Exit(1)
}
