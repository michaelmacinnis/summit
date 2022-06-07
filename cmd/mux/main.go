// Released under an MIT license. See LICENSE.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/creack/pty"
	"github.com/michaelmacinnis/summit/pkg/comms"
	"github.com/michaelmacinnis/summit/pkg/config"
	"github.com/michaelmacinnis/summit/pkg/errors"
	"github.com/michaelmacinnis/summit/pkg/message"
	"github.com/michaelmacinnis/summit/pkg/terminal"
)

type Status struct {
	pty  string
	rv   int
	term string
}

//nolint:gochecknoglobals
var (
	label  = "unknown"
	muxing = int32(0)
	status = 0
)

func logf(out chan [][]byte, format string, i ...interface{}) {
	out <- [][]byte{message.Log(label+": "+format, i...).Bytes()}
}

func session(id string, in chan *message.T, out chan [][]byte, statusq chan Status) {
	logf(out, "launching %s", id)

	// Find out what terminal this session is connected to.
	m := <-in
	term := m.Terminal()
	hdr := [][]byte{m.Bytes(), message.Pty(id)}

	// Find out what command we're running.
	m = <-in
	args := m.Args()

	cmd := exec.Command(args[0], args[1:]...) //nolint:gosec

	cmd.Env = m.Env()
	cmd.Dir = wd(cmd.Env)

	//logf(out, "environment: %#v", cmd.Env)

	f, err := pty.Start(cmd)
	if err != nil {
		println(err.Error())

		code := cmd.ProcessState.ExitCode()
		statusq <- Status{id, code, term}
	}

	defer func() {
		_ = f.Close() // Best effort.
	}()

	// println("session: started")

	fromProgram := comms.Chunk(f)
	toProgram := comms.Write(f)

	nested := int32(0)

	go func() {
		buffered := [][]byte{}

		for m := range in {
			var ws *pty.Winsize

			if m.Is(message.Escape) {
				ws = m.WindowSize()
				if ws != nil {
					if err := pty.Setsize(f, ws); err != nil {
						println("error setting window size:", err.Error())
					}
				} else if m.Command() != "run" {
					buffered = append(buffered, m.Bytes())
					continue
				}
			}

			if atomic.LoadInt32(&nested) > 0 {
				toProgram <- append(buffered, m.Bytes())
			} else if ws == nil {
				toProgram <- [][]byte{m.Bytes()}
			}


			buffered = [][]byte{}
		}
	}()

	go func() {
		buffered := append([][]byte{}, hdr...)

		sent := false

		for m := range fromProgram {
			if m.Is(message.Escape) {
				if m.Logging() {
					out <- [][]byte{m.Bytes()}

					continue
				}

				if sent {
					buffered = make([][]byte, len(hdr))
					copy(buffered, hdr)

					sent = false
				}

				terminal := m.Terminal()
				if terminal != "" {
					buffered[0] = m.Bytes()
				} else if m.Pty() != "" {
					buffered = append(buffered, m.Bytes())
				} else if n := int32(m.Mux()); n != 0 {
					atomic.AddInt32(&muxing, n)
					atomic.AddInt32(&nested, n)
				}

				if m.Command() != "mux" && m.Command() != "run" && m.Command() != "status" {
					continue
				}
			}

			// println("session: sending", m.String())

			// println("mux:", len(buffered))

			out <- append(buffered, m.Bytes())

			sent = true
		}
	}()

	err = cmd.Wait()
	if err != nil {
		println("error waiting for command: %s", err.Error())
	}

	code := cmd.ProcessState.ExitCode()

	statusq <- Status{id, code, term}
}

func wd(env []string) string {
	for _, s := range env {
		if strings.HasPrefix(s, "PWD=") {
			return strings.TrimPrefix(s, "PWD=")
		}
	}

	return ""
}

func main() {
	request := false

	flag.StringVar(&label, "l", label, "mux label (for debugging)")
	flag.BoolVar(&request, "n", request, "request new local session")
	flag.Parse()

	args, explicit := config.Command()
	// println("args", explicit, fmt.Sprintf("%v", args))

	if request {
		os.Stdout.Write(message.Run(args, os.Environ()))
		return
	}

	stream := map[string]chan *message.T{}
	term := ""

	next := comms.Counter(1)

	fromServer := comms.Chunk(os.Stdin)
	toServer := comms.Write(os.Stdout)

	toServer <- [][]byte{message.Mux(1)}
	defer func() {
		toServer <- [][]byte{message.Mux(-1)}

		errors.Exit(status)
	}()

	statusq := make(chan Status) // Pty ID + exit status.

	id := "0"

	if terminal.IsTTY() && explicit {
		println("launching", fmt.Sprintf("%v", args))
		// TODO: Launch as shim.
		c := make(chan *message.T)
		stream[id] = c

		restore, err := terminal.MakeRaw()
		errors.On(err).Die("failed to put terminal in raw mode")

		errors.AtExit(restore)

		go session(id, c, toServer, statusq)
		c <- message.New(message.Escape, message.Terminal(""))
		c <- message.New(message.Escape, message.Run(args, os.Environ()))
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
				logf(toServer, "mux command: %v", m.Parsed())

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

						go session(id, selected, toServer, statusq)
					}
				}
			} else {
				logf(toServer, "mux received: %v", strconv.Quote(string(m.Bytes())))
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

		case s := <-statusq:
			close(stream[s.pty])
			delete(stream, s.pty)

			if s.pty == "0" {
				status = s.rv
				term = s.term
			}

			toServer <- [][]byte{
				message.Terminal(s.term),
				message.Status(s.rv),
			}

			if len(stream) == 0 && atomic.LoadInt32(&muxing) == 0 {
				if term != "" {
					toServer <- [][]byte{
						message.Terminal(term),
						message.Status(status),
					}
				}

				return
			}
		}
	}
}
