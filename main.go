package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/ShashankRaoCoding/yozora/methods"

	// Register methods and their sub-methods via init().
	_ "github.com/ShashankRaoCoding/yozora/methods/convert"
	_ "github.com/ShashankRaoCoding/yozora/methods/convert/formats"
	_ "github.com/ShashankRaoCoding/yozora/methods/create"
	_ "github.com/ShashankRaoCoding/yozora/methods/ls"
	_ "github.com/ShashankRaoCoding/yozora/methods/print"
	_ "github.com/ShashankRaoCoding/yozora/methods/print/image"
	_ "github.com/ShashankRaoCoding/yozora/methods/print/table"
	_ "github.com/ShashankRaoCoding/yozora/methods/table"
)

func main() {
	args := os.Args[1:]
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: no method specified; available:", names())
		os.Exit(1)
	}

	m, args := args[0], args[1:]
	if f, ok := methods.Methods[m]; ok {
		f(args)
		return
	}
	fmt.Fprintln(os.Stderr, "Error: unconfigured method; available:", names())
	os.Exit(1)
}

func names() []string {
	var n []string
	for k := range methods.Methods {
		n = append(n, k)
	}
	sort.Strings(n)
	return n
}
