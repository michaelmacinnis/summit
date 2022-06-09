// Released under an MIT license. See LICENSE.

package terminal

import (
	"os"
	"os/signal"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

type WindowSize = pty.Winsize

func IsTTY() bool {
	return term.IsTerminal(int(stdin.Fd()))
}

func MakeRaw() (func() error, error) {
	fd := int(stdin.Fd())

	prev, err := term.MakeRaw(fd)

	return func() error {
		return term.Restore(fd, prev)
	}, err
}

func OnResize(f func(ws *WindowSize)) func() error {
	signal.Notify(signals, unix.SIGWINCH)

	go func() {
		for range signals {
			f(Size())
		}
	}()

	return func() error {
		signal.Stop(signals)
		close(signals)

		return nil
	}
}

func TriggerResize() {
	signals <- unix.SIGWINCH
}

func Size() *WindowSize {
    ws, err := pty.GetsizeFull(stdin)
    if err != nil {
        println("error getting window size:", err.Error())
        return nil
    }

	return ws
}

var (
	signals = make(chan os.Signal, 1)
	stdin   = os.Stdin
)
