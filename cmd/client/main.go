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
	"github.com/michaelmacinnis/summit/pkg/errors"
	"github.com/michaelmacinnis/summit/pkg/message"
	"github.com/michaelmacinnis/summit/pkg/terminal"
)

const (
	sock = "/tmp/summit.sock"
)

func main() {
	defer errors.Exit(0)

	restore, err := terminal.MakeRaw()
	errors.On(err).Die("failed to put terminal in raw mode")

	errors.AtExit(restore)

	c, err := net.Dial("unix", sock)
	errors.On(err).Die("failed to connect to server")

	errors.AtExit(c.Close)

	path := ""
	flag.StringVar(&path, "p", path, "routing path")
	flag.Parse()

	for _, s := range strings.Split(path, "-") {
		if s != "" {
			c.Write(message.Pty(s))
		}
	}

	c.Write(message.Run(flag.Args()))

	fromServer := comms.Chunk(c)
	fromTerminal := comms.Chunk(os.Stdin)

	toServer := c
	toTerminal := os.Stdout

	cleanup := terminal.ForwardResize(func (b []byte){
		toServer.Write(b)
	})
	errors.AtExit(cleanup)

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
			if m.Command() == "status" {
				errors.Exit(m.Status())
			}

			continue
		}

		s := m.Bytes()
		f.Write(s)

		newline = bytes.HasSuffix(s, message.CRLF)
	}
}
