// Released under an MIT license. See LICENSE.

package main

import (
	"bytes"
	"flag"
	"io"
	"net"
	"os"
	"strings"

	"github.com/michaelmacinnis/summit/pkg/comms"
	"github.com/michaelmacinnis/summit/pkg/config"
	"github.com/michaelmacinnis/summit/pkg/errors"
	"github.com/michaelmacinnis/summit/pkg/message"
	"github.com/michaelmacinnis/summit/pkg/terminal"
)

func main() {
	defer errors.Exit(0)

	muxing := int32(0)

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

	routing := [][]byte{}

	// Send routing information.
	for _, s := range strings.Split(*path, "-") {
		if s != "" {
			b := message.Pty(s)

			routing = append(routing, b)

			toServer.Write(b)
		}
	}

	// Send the command to run.
	args, _ := config.Command()
	toServer.Write(message.Run(args, config.Env(*j)))

	// Send the terminal size.
	toServer.Write(terminal.ResizeMessage().Bytes())

	// Continue to send terminal size changes.
	// These notifications are converted to look like terminal input so
	// that they are not interleaved with other output when writing.
	cleanup := terminal.ForwardResize(func(m *message.T) {
		fromTerminal <- m
	})
	errors.AtExit(cleanup)

	newline := false
	sent := true

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
			for _, b := range routing {
				toServer.Write(b)
			}

		case m = <-fromServer:
			if m == nil {
				goto done
			}

			f = toTerminal

			if m.Is(message.Escape) {
				if sent {
					routing = [][]byte{}

					sent = false
				}

				if m.IsPty() {
					routing = append(routing, m.Bytes())
				} else if n := int32(m.Mux()); n != 0 {
					muxing += n

					// Send routing information.
					for _, b := range routing {
						toServer.Write(b)
					}

					toServer.Write(terminal.ResizeMessage().Bytes())
				} else if muxing == 0 && m.IsStatus() {
					errors.Exit(m.Status())
				}

				continue
			}

		}

		s := m.Bytes()
		f.Write(s)

		newline = bytes.HasSuffix(s, message.CRLF)

		sent = true
	}

done:
	if !newline {
		toTerminal.Write(message.CRLF)
	}
}
