// Released under an MIT license. See LICENSE.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
//	"strconv"
	"strings"

	"github.com/michaelmacinnis/summit/pkg/comms"
	"github.com/michaelmacinnis/summit/pkg/config"
	"github.com/michaelmacinnis/summit/pkg/errors"
	"github.com/michaelmacinnis/summit/pkg/message"
)

func Write(wc io.WriteCloser) chan [][]byte {
	c := make(chan [][]byte)

	go func() {
		defer wc.Close()

		for bs := range c {
			for _, b := range bs {
				if b != nil {

/*
					// This block is just for debugging.
					s := strconv.Quote(string(b))
					if m := message.Deserialize(b); m != nil {
						s = fmt.Sprintf("%v", m)
					}
					println("TO MUX:", s)
*/

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
	client = config.Get("SUMMIT_CLIENT", "summit-client")
	mux    = config.Get("SUMMIT_MUX", "summit-mux")
	term   = config.Get("SUMMIT_TERMINAL", "./xfce-terminal")
)

func address(offset int, bs [][]byte) (string, string) {
	path := make([]string, len(bs))
	n := 0

	term := ""
	for _, b := range bs {
		m := message.New(message.Escape, b)
		switch {
		case m.IsPty():
			path[n] = m.Pty()
			n++

		case m.IsTerm():
			term = m.Term()
		}
	}

	n += offset
	if n > 0 {
		return term, strings.Join(path[:n], "-")
	}

	return term, ""
}

func dispatch(accepted <-chan net.Conn, fromMux chan *message.T, toMux chan [][]byte) {
	next := comms.Counter(1)

	terminals := map[string]chan *message.T{}

	var current chan *message.T
	var id string

	for {
		select {
		case conn := <-accepted:
			println("New terminal.")

			id = <-next

			fromDispatch := make(chan *message.T)
			terminals[id] = fromDispatch

			go terminal(id, conn, fromDispatch, toMux)

		case m, ok := <-fromMux:
			if !ok {
				return
			}

			if m.Is(message.Escape) {
				if m.Logging() {
					println("LOGGING:", m.Log())
					continue
				}

				if n := m.Term(); n != "" {
					id = n
					current = terminals[id]
				}
			}

			if current != nil {
				current <- m
			}
		}
	}
}

func launch(path string) (*exec.Cmd, chan *message.T, chan [][]byte) {
	cmd := exec.Command(path, "-l", "main")

	in, err := cmd.StdinPipe()
	errors.On(err).Die("stdin error")

	out, err := cmd.StdoutPipe()
	errors.On(err).Die("stdout error")

	cmd.Stderr = os.Stderr

	err = cmd.Start()
	errors.On(err).Die("start error")

	return cmd, comms.Chunk(out), Write(in)
}

func listen(accepted chan net.Conn) {
	os.Remove(config.Socket())

	l, err := net.Listen("unix", config.Socket())
	errors.On(err).Die("listen error")

	defer l.Close()

	for {
		conn, err := l.Accept()
		errors.On(err).Die("accept error")

		println("Got connection.")

		accepted <- conn
	}
}

func terminal(id string, conn net.Conn, fromMux <-chan *message.T, toMux chan [][]byte) {
	defer conn.Close()

	fromClient := comms.Chunk(conn)
	toClient := comms.Write(conn)

	output := [][]byte{}
	routing := [][]byte{}

	header := [][]byte{message.Term(id)}

	for {
		select {
		case m, ok := <-fromClient:
			if !ok {
				goto done
			}

			if m.Routing() {
				output = append(output, m.Bytes())
				continue
			}

			toMux <- append(append(header, output...), m.Bytes())

			// Clear program input buffer.
			output = [][]byte{}

		// From mux (after being demultiplexed by the dispatcher).
		case m, ok := <-fromMux:
			if !ok || m == nil {
				println("Channel closed or nil message.")
				goto done
			}

/*
			s := strconv.Quote(string(m.Bytes()))
			if m.Is(message.Escape) {
				s = fmt.Sprintf("%v", m.Parsed())
			}

			println("FROM MUX:", s)
*/

			if m.Routing() {
				routing = append(routing, m.Bytes())
				continue
			}

			if m.IsRun() {
				go window(m, routing)
				continue
			}

			toClient <- [][]byte{m.Bytes()}

			// Set header and clear routing.
			header = routing
			routing = [][]byte{}
		}
	}

done:
	// TODO: Write control message to delete terminal entry.
	return
}

func window(m *message.T, routing [][]byte) {
	args := []string{client}

	_, path := address(-1, routing)
	if path != "" {
		args = append(args, "-p", path)
	}

	j, err := json.Marshal(m.Env())
	if err != nil {
		println(err.Error())
	} else {
		args = append(args, "-e", string(j))
	}

	args = append(args, m.Args()...)

	println("REQUEST:", fmt.Sprintf("%s %v", term, args))

	cmd := exec.Command(term, args...)

	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		println(err.Error())
	}
}

func main() {
	flag.StringVar(&client, "c", client, "path to summit client")
	flag.StringVar(&mux, "m", mux, "path to summit mux")
	flag.StringVar(&term, "t", term, "path to terminal emulator")
	flag.Parse()

	accepted := make(chan net.Conn)

	// Listen for connections and send them to accepted.
	go listen(accepted)

	for {
		cmd, fromMux, toMux := launch(mux)

		go dispatch(accepted, fromMux, toMux)

		cmd.Wait()
	}
}
