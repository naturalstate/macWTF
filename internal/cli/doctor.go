package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/naturalstate/macWTF/internal/bootstrap"
)

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	rep := bootstrap.Check()
	var b strings.Builder
	rep.Render(&b)
	fmt.Print(b.String())

	if !rep.OK() {
		return fmt.Errorf("%d prerequisite(s) missing", len(rep.Blocking()))
	}
	return nil
}
