// Released under an MIT license. See LICENSE.

package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"

	"github.com/michaelmacinnis/summit/pkg/comms"
	"github.com/michaelmacinnis/summit/pkg/errors"
	"github.com/michaelmacinnis/summit/pkg/message"
	"github.com/michaelmacinnis/summit/pkg/session"
)

func Write(wc io.WriteCloser) chan [][]byte {
    c := make(chan [][]byte)

    go func() {
        defer wc.Close()

        for bs := range c {
            for _, b := range bs {
                if b != nil {
					s := strconv.Quote(string(b))
					if m := message.Deserialize(b); m != nil {
						s = fmt.Sprintf("%v", m)
					}
					println("TO MUX:", s)
                    _, err := wc.Write(b)
                    if err != nil {
                        println(err.Error())
                    }
                }
            }
        }
    }()

    return c
}

// TODO: allow overrides at the command line or with an environment variable.
var (
	addr = "/tmp/summit.sock"
	client = "summit-client"
	emulator = "kitty"
	mux = "summit-mux"
)

func launch(path string) (*exec.Cmd, chan *message.T, chan [][]byte) {
	cmd := exec.Command(path)

	in, err := cmd.StdinPipe()
	errors.On(err).Die("stdin error")

	out, err := cmd.StdoutPipe()
	errors.On(err).Die("stdout error")

	cmd.Stderr = os.Stderr

	err = cmd.Start()
	errors.On(err).Die("start error")

	//return cmd, comms.Chunk(out), comms.Write(in)
	return cmd, comms.Chunk(out), Write(in)
}

func listen(accepted chan net.Conn) {
	os.Remove(addr)

    l, err := net.Listen("unix", addr)
    errors.On(err).Die("listen error")

    defer l.Close()

    for {
        conn, err := l.Accept()
        errors.On(err).Die("accept error")

		accepted <- conn
    }
}

func loop(accepted chan net.Conn, r chan *message.T, w chan [][]byte) {
	next := comms.Counter(1)

	terminals := map[string]chan *message.T{}

	var current chan *message.T

	for {
		select {
		case conn := <-accepted:
			id := <-next
			ch := make(chan *message.T)
			terminals[id] = ch

			go terminal(id, conn, r, ch, w)

		case m := <-r:
			if m.Is(message.Escape) {
				if m.Logging() {
					println("LOGGING:", m.Log())
					continue
				}

				if id := m.Terminal(); id != "" {
					current = terminals[id]
				}
			}

			if current != nil {
				current <- m
			}
		}
	}
}

func terminal(id string, conn net.Conn, ctl, r chan *message.T, w chan [][]byte) {
	defer conn.Close()

	display := comms.Write(conn)
	keyboard := comms.Chunk(conn)

	output := [][]byte{}
	routing := [][]byte{}

	header := [][]byte{message.Terminal(id)}

	for {
		select {
		case m, ok := <-keyboard:
			if !ok {
				goto done
			}

			if m.Routing() {
				output = append(output, m.Bytes())
				continue
			}

			w <- append(append(header, output...), m.Bytes())

			// Clear program input buffer.
			output = [][]byte{}
		case m, ok := <-r:
			if !ok {
				goto done
			}

			s := strconv.Quote(string(m.Bytes()))
			if m.Is(message.Escape) {
				s = fmt.Sprintf("%v", m.Parsed())
			}

			println("FROM MUX:", s)

			if m.Routing() {
				routing = append(routing, m.Bytes())
				continue
			}

			send := false
			if m.Command() == "run" {
				go window(m, routing)
			} else if m.Command() != "status" {
				send = session.Track(routing)
			} else {
				var bs [][]byte
				send, bs = session.Remove(routing)
				if len(bs) > 0 {
					w <- append(bs, m.Bytes())
				}
				if len(routing) > 1 {
					routing = routing[:len(routing)-1]
				}
			}

			if send {
				display <- [][]byte{m.Bytes()}
			}

			// Set header and clear routing.
			header = session.Valid(routing)
			routing = [][]byte{}
		}
	}

done:
	// TODO: Write control message to delete terminal entry.
	return
}

func window(m *message.T, routing [][]byte) {
	args := []string{client}

	_, path := session.Path(-1, routing)
	if path != "" {
		args = append(args, "-p", path)
	}
	args = append(args, m.Args()...)

	println("REQUEST:", fmt.Sprintf("%s %v", emulator, args))

    cmd := exec.Command(emulator, args...)

    cmd.Stderr = os.Stderr

    err := cmd.Run()
    if err != nil {
        println(err.Error())
    }
}

func main() {
	cmd, r, w := launch(mux)

	accepted := make(chan net.Conn)
	go listen(accepted)

	go loop(accepted, r, w)

	cmd.Wait()

	os.Exit(cmd.ProcessState.ExitCode())
}

