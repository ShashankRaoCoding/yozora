// Package print implements `yozora print <method>`.
package print

import (
	"fmt"
	"os"
	"sort"

	"github.com/ShashankRaoCoding/yozora/methods"
)

// Methods maps a print sub-method name to its entry point.
var Methods = map[string]func([]string){}

func init() {
	methods.Methods["print"] = print
}

func print(args []string) {
	var avail []string
	for k := range Methods {
		avail = append(avail, k)
	}
	sort.Strings(avail)

	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: no method specified; available:", avail)
		os.Exit(1)
	}
	m, args := args[0], args[1:]
	if f, ok := Methods[m]; ok {
		f(args)
		return
	}
	fmt.Fprintln(os.Stderr, "Error: unconfigured method; available:", avail)
	os.Exit(1)
}
