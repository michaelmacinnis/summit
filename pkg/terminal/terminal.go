// Released under an MIT license. See LICENSE.

package terminal

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"

	"github.com/michaelmacinnis/summit/pkg/message"
)

var signals chan os.Signal

func ForwardResize(f func(m *message.T)) func() error {
	fd := os.Stdin

	signals = make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGWINCH)

	go func() {
		for range signals {
			ws, err := pty.GetsizeFull(fd)
			if err != nil {
				fmt.Printf("error getting window size: %s\n", err.Error())
				continue
			}

			f(message.KV(map[string]interface{}{
				"cmd": "set-window-size",
				"ws":  &ws,
			}))
		}
	}()

	return func() error {
		signal.Stop(signals)
		close(signals)

		return nil
	}
}

func IsTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func MakeRaw() (func() error, error) {
	fd := int(os.Stdin.Fd())

	prev, err := term.MakeRaw(fd)

	return func() error {
		return term.Restore(fd, prev)
	}, err
}

func Sigwinch() {
	signals <- syscall.SIGWINCH
}
