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
	flag.Parse()

	restore, err := terminal.MakeRaw()
	errors.On(err).Die("failed to put terminal in raw mode")

	errors.AtExit(restore)

	c, err := net.Dial("unix", config.Socket())
	errors.On(err).Die("failed to connect to server")

	errors.AtExit(c.Close)

	for _, s := range strings.Split(*path, "-") {
		if s != "" {
			c.Write(message.Pty(s))
		}
	}

	args, _ := config.Command()
	c.Write(message.Run(args, config.Env(*j)))

	fromServer := comms.Chunk(c)
	fromTerminal := comms.Chunk(os.Stdin)

	toServer := c
	toTerminal := os.Stdout

	cleanup := terminal.ForwardResize(func(m *message.T) {
		fromTerminal <- m
	})
	errors.AtExit(cleanup)

	go terminal.Sigwinch()

	newline := false
	for {
		var f io.Writer
		var m *message.T

		select {
		case m = <-fromTerminal:
			f = toServer
		case m = <-fromServer:
			f = toTerminal
		}

		if m == nil {
			if !newline {
				toTerminal.Write(message.CRLF)
			}
			break
		}

		if m.Is(message.Escape) {
			if n := int32(m.Mux()); n != 0 {
				muxing += n
			} else if muxing == 0 && m.Command() == "status" {
				errors.Exit(m.Status())
			}

			if m.Command() != "set-window-size" {
				continue
			}
		}

		s := m.Bytes()
		f.Write(s)

		newline = bytes.HasSuffix(s, message.CRLF)
	}
}
