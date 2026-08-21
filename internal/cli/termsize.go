package cli

import (
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// termSize returns the terminal dimensions of stdout.
func termSize() (width, height int, err error) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0, err
	}
	_ = unsafe.Sizeof(ws)
	return int(ws.Col), int(ws.Row), nil
}
