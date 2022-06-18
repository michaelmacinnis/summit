// Released under an MIT license. See LICENSE.

package terminal

import (
	"os"
	"os/signal"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

type Size = pty.Winsize

//nolint:gochecknoglobals
var (
	SetSize       = pty.Setsize
	StartWithSize = pty.StartWithSize
)

func GetSize() *Size {
	ts, err := pty.GetsizeFull(stdin)
	if err != nil {
		println("error getting window size:", err.Error())

		return nil
	}

	return ts
}

func IsTTY() bool {
	return term.IsTerminal(int(stdin.Fd()))
}

func MakeRaw() (func(), error) {
	fd := int(stdin.Fd())

	prev, err := term.MakeRaw(fd)

	return func() {
		_ = term.Restore(fd, prev)
	}, err
}

func OnResize(f func(ts *Size)) func() {
	signals := make(chan os.Signal, 1)

	signal.Notify(signals, unix.SIGWINCH)

	go func() {
		for range signals {
			f(GetSize())
		}
	}()

	return func() {
		signal.Stop(signals)
		close(signals)
	}
}

var stdin = os.Stdin //nolint:gochecknoglobals
