// Released under an MIT license. See LICENSE.

package terminal

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"

	"github.com/michaelmacinnis/summit/pkg/message"
)

func ForwardResize(f func(m *message.T)) func() error {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGWINCH)

	go func() {
		for range signals {
			f(ResizeMessage())
		}
	}()

	return func() error {
		signal.Stop(signals)
		close(signals)

		return nil
	}
}

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

func ResizeMessage() *message.T {
	return message.KV(map[string]interface{}{
		"cmd": "set-window-size",
		"ws":  WindowSize(),
	})
}

func WindowSize() *pty.Winsize {
    ws, err := pty.GetsizeFull(stdin)
    if err != nil {
        println("error getting window size:", err.Error())
        return nil
    }

	return ws
}

var stdin = os.Stdin
