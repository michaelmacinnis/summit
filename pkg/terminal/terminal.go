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

func OnResize(f func()) func() error {
	signal.Notify(signals, syscall.SIGWINCH)

	go func() {
		for range signals {
			f()
		}
	}()

	return func() error {
		signal.Stop(signals)
		close(signals)

		return nil
	}
}

func ResizeMessage() *message.T {
	return message.KV(map[string]interface{}{
		"cmd": "ws",
		"ws":  WindowSize(),
	})
}

func TriggerResize() {
	signals <- syscall.SIGWINCH
}

func WindowSize() *pty.Winsize {
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
