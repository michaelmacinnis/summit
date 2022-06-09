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

	// Send routing.
	for _, s := range strings.Split(*path, "-") {
		if s != "" {
			b := message.Pty(s)

			toServer.Write(b)
		}
	}

	// Send the command to run.
	args, _ := config.Command()
	toServer.Write(message.Run(args, config.Env(*j), terminal.WindowSize()))

	// Continue to send terminal size changes.
	// These notifications are converted to look like terminal input so
	// that they are not interleaved with other output when writing.
	cleanup := terminal.OnResize(func() {
		fromTerminal <- terminal.ResizeMessage()
	})
	errors.AtExit(cleanup)

	buf := buffer.New()

	for buf.Message(<-fromServer) {}

	newline := false

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

			if n := int32(m.Mux()); n != 0 {
				muxing += n

				continue
			}

			if buf.Message(m) {
				continue
			}

			if m.IsStatus() {
				if muxing == 0 {
					errors.Exit(m.Status())
				}

				buf.Remove()

				terminal.TriggerResize()

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
