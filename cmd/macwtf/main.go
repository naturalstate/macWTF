// Command macwtf installs the pentesting, InfoSec, dev and utility tooling
// that does not ship with macOS.
package main

import (
	"fmt"
	"os"

	"github.com/naturalstate/macWTF/internal/cli"
)

// version is overridden at build time via -ldflags.
var version = "0.1.0-dev"

func main() {
	if err := cli.Run(os.Args[1:], version); err != nil {
		fmt.Fprintln(os.Stderr, "error: "+err.Error())
		os.Exit(1)
	}
}
