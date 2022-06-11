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
	"github.com/michaelmacinnis/summit/pkg/buffer"
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
	// First message should be the terminal for this session.
	logf(out, "[%s] getting terminal id", id)

	term := <-in

	logf(out, "[%s] got terminal id %s:%s", id, term.Command(), term.Term())

	// Second message should be the command, environment, and window size.
	m := <-in
	args := m.Args()
	ws   := m.WindowSize()

	logf(out, "[%s] sending new pty id", id)
	out <- [][]byte{term.Bytes(), message.Pty(id), message.Started()}

	cmd := exec.Command(args[0], args[1:]...) //nolint:gosec

	cmd.Env = m.Env()
	cmd.Dir = wd(cmd.Env)

	logf(out, "[%s] launching %#v (%#v)", id, args, cmd.Env)

	// Always send a status message on completion.
	defer func() {
		statusq <- Status{id, cmd.ProcessState.ExitCode(), term.Term()}
	}()

	f, err := pty.Start(cmd)
	if err != nil {
		logf(out, "[%s] error: launching: %s", id, err.Error())

		return
	}

	defer func() {
		_ = f.Close() // Best effort.
	}()

	if ws != nil {
		if err := pty.Setsize(f, ws); err != nil {
			logf(out, "[%s] error: setting size: %s", id, err.Error())
		}
	}

	fromProgram := comms.Chunk(f)
	fromTerminal := in
	toProgram := comms.Write(f)
	toTerminal := out

	go func() {
		buf := buffer.New(term)

		for m := range fromTerminal {
			if buf.Message(m) {
				continue
			}

			routing := buf.Routing()
			if len(routing) == 1 && !m.IsRun() {
				ws := m.WindowSize()
				if ws != nil {
					if err := pty.Setsize(f, ws); err != nil {
						logf(out, "[%s] error: setting size: %s", id, err.Error())
					}
				} else {
					toProgram <- [][]byte{m.Bytes()}
				}
			} else {
				toProgram <- append(routing, m.Bytes())
			}
		}
	}()

	go func() {
		buf := buffer.New(term, message.Command(message.Pty(id)))
		buf.IgnoreBlankTerm = true

		for m := range fromProgram {
			if m.Logging() {
				toTerminal <- [][]byte{m.Bytes()}

				continue
			}

			if n := int32(m.Mux()); n != 0 {
				atomic.AddInt32(&muxing, n)
			}

			if buf.Message(m) {
				continue
			}

			toTerminal <- append(buf.Routing(), m.Bytes())
		}
	}()

	_ = cmd.Wait()
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

	flag.Usage = func() {
		f := flag.CommandLine.Output()
		fmt.Fprintf(f, "%s\n\nUsage:\n", os.Args[0])
		fmt.Fprintf(f, "  %s [-l LABEL] COMMAND ARGUMENTS...\n", os.Args[0])
		fmt.Fprintf(f, "  %s -n\n\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.StringVar(&label, "l", label, "mux label (for debugging)")
	flag.BoolVar(&request, "n", request, "request new local session")
	flag.Parse()

	args, defaulted := config.Command()

	if request {
		os.Stdout.Write(message.Run(args, os.Environ(), nil))
		return
	} else if defaulted && terminal.IsTTY() {
		flag.Usage()

		errors.Exit(1)
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

	if terminal.IsTTY() {
		println("launching", fmt.Sprintf("%v", args))

		c := make(chan *message.T)
		stream[id] = c

		restore, err := terminal.MakeRaw()
		errors.On(err).Die("failed to put terminal in raw mode")

		errors.AtExit(restore)

		go session(id, c, toServer, statusq)
		c <- message.Command(message.Term(""))
		c <- message.Command(message.Run(args, os.Environ(), terminal.Size()))
	}

	routing := []*message.T{}

	var selected chan *message.T

	for {
		select {
		case m := <-fromServer:
			if m == nil {
				return
			}

			if m.Is(message.Escape) {
				logf(toServer, "mux command: %v", m.Parsed())

				switch {
				case m.IsPty():
					if selected == nil {
						id = m.Pty()
						selected = stream[id]
					} else {
						routing = append(routing, m)
					}

					continue

				case m.IsTerm():
					routing = []*message.T{m}

					continue

				case m.IsRun():
					if selected == nil {
						id = <-next
						selected = make(chan *message.T)

						stream[id] = selected

						go session(id, selected, toServer, statusq)

						// Send resize message first.
						// selected <- <-fromServer
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

			for _, b := range routing {
				selected <- b
			}

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
				message.Term(s.term),
				message.Pty(s.pty),
				message.Status(s.rv),
			}

			if len(stream) == 0 && atomic.LoadInt32(&muxing) == 0 {
				if term != "" {
					toServer <- [][]byte{
						message.Term(term),
						message.Pty("0"),
						message.Status(status),
					}
				}

				return
			}
		}
	}
}
