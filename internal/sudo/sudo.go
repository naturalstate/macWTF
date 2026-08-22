// Package sudo handles administrator authorisation for a run.
//
// macOS differs from Linux here in a way that matters. You do not run the
// installer under sudo — Homebrew refuses to run as root — so elevation is
// requested per-command, deep inside a run, by whichever cask happens to ship a
// .pkg payload. A password prompt appearing partway through is bad enough on a
// plain terminal; inside a full-screen interface it writes over the display.
//
// So authorisation is primed once, before anything starts, and kept warm for
// the duration. The user types their password at a predictable moment, or
// declines and the run proceeds without the steps that need it.
package sudo

import (
	"context"
	"os"
	"os/exec"
	"time"
)

// Available reports whether sudo exists at all.
func Available() bool {
	_, err := exec.LookPath("sudo")
	return err == nil
}

// Active reports whether a valid sudo timestamp already exists, so no password
// would be required right now. Uses -n so that asking cannot itself prompt.
func Active() bool {
	if !Available() {
		return false
	}
	return exec.Command("sudo", "-n", "-v").Run() == nil
}

// PrimeCmd returns the command that asks for the password.
//
// Returned rather than run so the caller can decide how to attach it to the
// terminal. A TUI must suspend itself first; a plain CLI can run it directly.
func PrimeCmd() *exec.Cmd {
	cmd := exec.Command("sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// Prime asks for the password now, attached to the current terminal.
func Prime() error { return PrimeCmd().Run() }

// KeepAlive refreshes the sudo timestamp until the context is cancelled.
//
// The default timeout is five minutes, and a large install runs far longer than
// that, so without this the password would be demanded again halfway through —
// reintroducing exactly the interruption priming was meant to remove.
func KeepAlive(ctx context.Context) {
	if !Available() {
		return
	}
	go func() {
		t := time.NewTicker(60 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				// -n so a lapsed timestamp fails silently here
				// rather than blocking on a prompt nobody is
				// watching.
				_ = exec.Command("sudo", "-n", "-v").Run()
			}
		}
	}()
}
