// Package methods holds the registry of top-level yozora commands.
package methods

// Methods maps a command name (e.g. "print") to its entry point.
var Methods = map[string]func([]string){}

func init() {

}
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
	_ "github.com/ShashankRaoCoding/yozora/methods/ensure"
	_ "github.com/ShashankRaoCoding/yozora/methods/ls"
	_ "github.com/ShashankRaoCoding/yozora/methods/print"
	_ "github.com/ShashankRaoCoding/yozora/methods/print/image"
	_ "github.com/ShashankRaoCoding/yozora/methods/print/table"
	_ "github.com/ShashankRaoCoding/yozora/methods/table"
)
