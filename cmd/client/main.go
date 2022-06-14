// Released under an MIT license. See LICENSE.

package main

import (
	"bytes"
	"flag"
	"io"
	"net"
	"os"
	"strings"

	"github.com/michaelmacinnis/summit/pkg/buffer"
	"github.com/michaelmacinnis/summit/pkg/comms"
	"github.com/michaelmacinnis/summit/pkg/config"
	"github.com/michaelmacinnis/summit/pkg/errors"
	"github.com/michaelmacinnis/summit/pkg/message"
	"github.com/michaelmacinnis/summit/pkg/terminal"
)

func resize(w io.Writer, buf *buffer.T, n int) {
	routing := buf.Routing()

	size := len(routing) + n
	if size <= 0 {
		return
	}

	for _, b := range routing[:size] {
		w.Write(b)
	}

	w.Write(message.TerminalSize(terminal.GetSize()))
}

func main() {
	defer errors.Exit(0)

	j := flag.String("e", "", "environment (as a JSON array)")
	path := flag.String("p", "", "routing path")
	config.Parse()

	restore, err := terminal.MakeRaw()
	errors.On(err).Die("failed to put terminal in raw mode")

	errors.AtExit(restore)

	c, err := net.Dial("unix", config.Socket())
	errors.On(err).Die("failed to connect to server")

	errors.AtExit(c.Close)

	fromServer := comms.Chunk(c)
	fromTerminal := comms.Chunk(os.Stdin)
	toServer := c
	toTerminal := os.Stdout

	// Send routing.
	for _, s := range strings.Split(*path, "-") {
		if s != "" {
			b := message.Pty(s)

			toServer.Write(b)
		}
	}

	// Send the command to run.
	args, _ := config.Command()
	toServer.Write(message.Run(args, config.Env(*j)))

	buf := buffer.New()

	// Wait for started message.
	m := <-fromServer
	for buf.Buffered(m) {
		m = <-fromServer
	}

	if !m.IsStarted() {
		s := "nil"
		if m != nil {
			s = m.String()
		}

		println("expected started message got", s)

		return
	}

	// Send terminal size.
	resize(toServer, buf, 0)

	// Continue to send terminal size changes.
	// These notifications are converted to look like terminal input so
	// that they are not interleaved with other output when writing.
	terminal.OnResize(func(ts *terminal.Size) {
		fromTerminal <- message.Raw(message.TerminalSize(ts))
	})

	newline := false
	running := 1

	for {
		var f io.Writer
		var m *message.T

		select {
		case m = <-fromTerminal:
			if m == nil {
				goto done
			}

			f = toServer

			// Send routing information.
			for _, b := range buf.Routing() {
				toServer.Write(b)
			}

		case m = <-fromServer:
			if m == nil {
				goto done
			}

			if buf.Buffered(m) {
				continue
			}

			if m.Is(message.Command) {
				if m.IsStarted() {
					running++

					resize(toServer, buf, 0)
				} else if m.IsStatus() {
					running--
	
					if running == 0 {
						errors.Exit(m.Status())
					}
	
					resize(toServer, buf, -1)
				}

				// Unexpected message. Don't send to terminal.
				continue
			}

			f = toTerminal

		}

		s := m.Bytes()
		f.Write(s)

		newline = bytes.HasSuffix(s, message.CRLF)
	}

done:
	if !newline {
		toTerminal.Write(message.CRLF)
	}
}
