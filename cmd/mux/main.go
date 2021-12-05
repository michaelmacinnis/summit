// Released under an MIT license. See LICENSE.

package main

import (
	"flag"
    "os"
    "os/exec"
    "github.com/creack/pty"

    "github.com/michaelmacinnis/summit/pkg/comms"
    "github.com/michaelmacinnis/summit/pkg/errors"
    "github.com/michaelmacinnis/summit/pkg/message"
    "github.com/michaelmacinnis/summit/pkg/terminal"
)

type Status struct {
    id string
    rv int
}

var label = "unknown"

func log(out chan [][]byte, format string, i ...interface{}) {
	out <- [][]byte{message.Log(label + ": " + format, i...).Bytes()}
}

func session(id string, in chan *message.T, out chan [][]byte, status chan Status) {
	log(out, "launching %s", id)

	m := <-in
	hdr := [][]byte{m.Bytes(), message.Pty(id)}

	m = <-in
	args := m.Args()

	cmd := exec.Command(args[0], args[1:]...)
	f, err := pty.Start(cmd)
	if err != nil {
		println(err.Error())

		code := cmd.ProcessState.ExitCode()
		status <- Status{id, code}

		out <-append(hdr[:1], message.Status(code))
	}

	defer func() {
		_ = f.Close() // Best effort.
	}()

	//println("session: started")

	fromProgram := comms.Chunk(f)
	toProgram := comms.Write(f)

	go func() {
		buffered := make([][]byte, len(hdr))
		copy(buffered, hdr)

		sent := false

		for m := range fromProgram {
			if m.Is(message.Escape) && m.Command() != "status" {
				if sent {
					buffered = make([][]byte, len(hdr))
					copy(buffered, hdr)
					sent = false
				}

				if m.Logging() {
					out <- [][]byte{m.Bytes()}
					continue
				}

				terminal := m.Terminal()
				if terminal != "" {
					buffered[0] = m.Bytes()
				} else if m.Pty() != "" {
					buffered = append(buffered, m.Bytes())
				}

				if m.Command() != "run" {
					continue
				}
			}

			//println("session: sending", m.String())

			//println("mux:", len(buffered))

			out <- append(buffered, m.Bytes())
			sent = true

			if m.Command() == "status" {
				buffered = make([][]byte, len(hdr))
				copy(buffered, hdr)
				sent = false
			}
		}
	}()

	go func() {
		buffered := [][]byte{}
		routing := 0

		for m := range in {
			//println("session: received", m.String())

			if m.Is(message.Escape) {
				if m.Command() != "run" && m.Command() != "status" {
					buffered = append(buffered, m.Bytes())
				}

				if m.Command() == "pty" {
					routing++
				}

				ws := m.WindowSize()
				if ws != nil {
					if err := pty.Setsize(f, ws); err != nil {
						println("error setting window size:", err.Error())
					}

				}

				if m.Command() != "run" && m.Command() != "status" {
					continue
				}
			}

			if routing > 0 || m.Command() == "run" || m.Command() == "status" {
				toProgram <-append(buffered, m.Bytes())
			} else {
				toProgram <-[][]byte{m.Bytes()}
			}

			buffered = [][]byte{}
			routing = 0
		}
	}()

	cmd.Wait()

	code := cmd.ProcessState.ExitCode()
	status <- Status{id, code}

	out <-append(hdr, message.Status(code))
}

func main() {
	flag.StringVar(&label, "l", label, "mux label (for debugging)")
	flag.Parse()

	args := flag.Args()
    if len(args) > 1 && args[0] == "new" {
		os.Stdout.Write(message.Run(args[1:]))
		return
	}

	defer errors.Exit(0)

	stream := map[string]chan *message.T{}

	next := comms.Counter(1)

	fromServer := comms.Chunk(os.Stdin)
	toServer := comms.Write(os.Stdout)

	status := make(chan Status) // Pty ID + exit status.

	id := "0"

	if terminal.IsTTY() && len(args) > 0 {
		// TODO: Launch as shim.
		c := make(chan *message.T)
		stream[id] = c

		restore, err := terminal.MakeRaw()
		errors.On(err).Die("failed to put terminal in raw mode")

		errors.AtExit(restore)

		go session(id, c, toServer, status)
		c <- message.New(message.Escape, message.Terminal(""))
		c <- message.New(message.Escape, message.Run(args))

		cleanup := terminal.ForwardResize(func(b []byte) {
			c <- message.New(message.Escape, b)
		})
		errors.AtExit(cleanup)
	}

	buffered := []*message.T{}

	var selected chan *message.T

	for {
		select {
			case m := <-fromServer:
				if m == nil {
					return
				}

				if m.Is(message.Escape) {
					log(toServer, "mux received: %v", m.Parsed())

					switch m.Command() {
					case "pty":
						if selected == nil {
							id = m.Pty()
							selected = stream[id]
							continue
						}

						fallthrough
					case "term":
						buffered = append(buffered, m)

						continue

					case "run":
						if selected == nil {
							id = <-next
							selected = make(chan *message.T)

							stream[id] = selected

							for _, v := range buffered {
								log(toServer, "buffered: %v", v.Parsed())
							}

							go session(id, selected, toServer, status)
/*
						} else {
							buffered = append(buffered, m)
							continue
*/
						}

					case "status":
						if selected == nil {
							errors.Exit(m.Status())
						}
					}
				}

				if selected == nil {
					selected = stream[id]
					if selected == nil {
						continue
					}
				}

				for _, b := range buffered {
					selected <- b
				}
				buffered = []*message.T{}

				selected <- m

				selected = nil

			case s := <-status:
				close(stream[s.id])
				delete(stream, s.id)
		}
	}
}
