// Released under an MIT license. See LICENSE.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/michaelmacinnis/summit/pkg/buffer"
	"github.com/michaelmacinnis/summit/pkg/comms"
	"github.com/michaelmacinnis/summit/pkg/config"
	"github.com/michaelmacinnis/summit/pkg/errors"
	"github.com/michaelmacinnis/summit/pkg/message"
	"github.com/michaelmacinnis/summit/pkg/terminal"
)

type Status struct {
	n    int
	pty  string
	rv   int
	term string
}

//nolint:gochecknoglobals
var (
	debug  = true
	label  = "unknown"
	nested = 0
	status = 0
)

func logf(out chan [][]byte, format string, i ...interface{}) {
	if debug {
		out <- [][]byte{message.Log(label+": "+format, i...).Bytes()}
	}
}

func session(id string, in chan *message.T, out chan [][]byte, statusq chan *Status) {
	// First message should be the terminal for this session.
	logf(out, "[%s] getting terminal id", id)

	term := <-in

	logf(out, "[%s] got terminal id %s:%s", id, term.Command(), term.Term())

	// Second message should be the command, environment, and window size.
	m := <-in
	args := m.Args()
	ts := m.TerminalSize()

	// TODO: When launched as a shim for a command that does not exist,
	//       this triggers a resize message but this mux is not longer
	//       here to receive it.
	logf(out, "[%s] sending new pty id", id)
	out <- [][]byte{term.Bytes(), message.Pty(id), message.Started()}

	cmd := exec.Command(args[0], args[1:]...) //nolint:gosec

	cmd.Env = m.Env()
	cmd.Dir = wd(cmd.Env)

	logf(out, "[%s] launching %#v (%#v)", id, args, cmd.Env)

	actual := term

	// Always send a status message on completion.
	defer func() {
		statusq <- &Status{0, id, cmd.ProcessState.ExitCode(), actual.Term()}
	}()

	f, err := terminal.Start(cmd)
	if err != nil {
		logf(out, "[%s] error: launching: %s", id, err.Error())

		return
	}

	defer func() {
		_ = f.Close() // Best effort.
	}()

	if ts != nil {
		if err := terminal.SetSize(f, ts); err != nil {
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
			if m.IsTerm() {
				actual = m
			}

			if buf.Buffered(m) {
				continue
			}

			routing := buf.Routing()
			if len(routing) == 1 && !m.IsRun() {
				ts := m.TerminalSize()
				if ts != nil {
					if err := terminal.SetSize(f, ts); err != nil {
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
		buf := buffer.New(term, message.Raw(message.Pty(id)))
		buf.IgnoreBlankTerm = true

		for m := range fromProgram {
			if m.Logging() {
				toTerminal <- [][]byte{m.Bytes()}

				continue
			}

			if buf.Buffered(m) {
				continue
			}

			if m.IsStarted() {
				statusq <- &Status{n: 1}
			} else if m.IsStatus() {
				statusq <- &Status{n: -1}
			}

			bs := append(buf.Routing(), m.Bytes())
			logf(toTerminal, "mux sent {")
			for _, b := range bs {
				logf(toTerminal, "mux sent: %s", message.Raw(b))
			}
			logf(toTerminal, "}")

			toTerminal <- bs
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

	done := make(chan struct{})
	next := comms.Counter(1)

	fromServer := comms.Chunk(os.Stdin)
	toServer := comms.Write(os.Stdout, done)

	defer func() {
        close(toServer)
        <-done

		errors.Exit(status)
	}()

	statusq := make(chan *Status) // Pty ID + exit status.

	id := "0"

	if terminal.IsTTY() {
		println("launching", fmt.Sprintf("%v", args))

		c := make(chan *message.T)
		stream[id] = c

		restore, err := terminal.MakeRaw()
		errors.On(err).Die("failed to put terminal in raw mode")

		errors.AtExit(restore)

		go session(id, c, toServer, statusq)
		c <- message.Raw(message.Term(""))
		c <- message.Raw(message.Run(args, os.Environ(), terminal.GetSize()))
	}

	routing := []*message.T{}

	var selected chan *message.T

	for {
		select {
		case m := <-fromServer:
			if m == nil {
				return
			}

			logf(toServer, "mux recv: %s", m)

			if m.Is(message.Escape) {
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
					}
				}
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
			logf(toServer, "status len(stream)=%d, nested=%d, n=%d, term='%s', pty='%s', rv=%d", len(stream), nested, s.n, s.term, s.pty, s.rv)

			if s.n != 0 {
				nested += s.n

				continue
			}

			close(stream[s.pty])
			delete(stream, s.pty)

			if s.pty == "0" {
				status = s.rv
				term = s.term
			} else {
				toServer <- [][]byte{
					message.Term(s.term),
					message.Pty(s.pty),
					message.Status(s.rv),
				}
			}

			if nested == 0 && len(stream) == 0 {
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
