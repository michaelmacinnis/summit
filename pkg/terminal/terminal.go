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

func ForwardResize(f func (b []byte)) func() error {
	fd := os.Stdin

    signals := make(chan os.Signal, 1)
    signal.Notify(signals, syscall.SIGWINCH)

    go func() {
        for range signals {
            ws, err := pty.GetsizeFull(fd)
            if err != nil {
                fmt.Printf("error getting window size: %s\n", err.Error)
                continue
            }

			f(message.Serialize(map[string]interface{}{
				"cmd": "set-window-size",
				"ws":  &ws,
			}))
        }
    }()

    signals <- syscall.SIGWINCH

    return func() error {
		signal.Stop(signals)
		close(signals)

		return nil
	}
}

func MakeRaw() (func() error, error) {
	fd := int(os.Stdin.Fd())

	prev, err := term.MakeRaw(fd)

	return func() error {
		return term.Restore(fd, prev)
	}, err
}

func IsTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
