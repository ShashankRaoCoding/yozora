// Package methods holds the registry of top-level yozora commands.
package methods

// Methods maps a command name (e.g. "print") to its entry point.
var Methods = map[string]func([]string){}
