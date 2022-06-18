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
	debug = true
	label = "unknown"
)

func logf(out chan [][]byte, format string, i ...interface{}) {
	if debug {
		out <- [][]byte{message.Log(label+": "+format, i...).Bytes()}
	}
}

func session(id string, in chan *message.T, out chan [][]byte, statusq chan *Status) {
	defer func() {
		r := recover()
		if r != nil {
			println(fmt.Sprintf("unexpected error: %v", r))
		}
	}()

	// First message should be the terminal for this session.
	logf(out, "[%s] getting terminal id", id)

	term := <-in

	logf(out, "[%s] got terminal id %s:%s", id, term.Command(), term.Term())

	// Second message should be the command, environment, and window size.
	m := <-in
	args := m.Args()

	logf(out, "[%s] sending new pty id", id)
	out <- [][]byte{term.Bytes(), message.Pty(id), message.Started()}

	cmd := exec.Command(args[0], args[1:]...) //nolint:gosec

	cmd.Env = m.Env()
	cmd.Dir = wd(cmd.Env)

	// Third message should be the terminal size.
	m = <-in
	ts := m.TerminalSize()

	logf(out, "[%s] launching %#v (%#v)", id, args, cmd.Env)

	// Always send a status message on completion.
	defer func() {
		statusq <- &Status{0, id, cmd.ProcessState.ExitCode(), term.Term()}
	}()

	f, err := terminal.StartWithSize(cmd, ts)
	if err != nil {
		logf(out, "[%s] error: launching: %s", id, err.Error())
		if id == "0" {
			println(err.Error() + "\r")
		}

		return
	}

	defer func() {
		_ = f.Close() // Best effort.
	}()

	dst := buffer.New(term)
	fromProgram := comms.Chunk(f)
	fromTerminal := in
	nested := 0
	src := buffer.New(term, message.Raw(message.Pty(id)))
	toProgram := comms.Write(f)
	toTerminal := out

	for {
		select {
		case m, ok := <-fromTerminal:
			if !ok || m == nil {
				goto done
			}

			if dst.Buffered(m) {
				continue
			}

			routing := dst.Routing()
			if len(routing) > 1 || m.IsRun() {
				if nested == 0 {
					logf(out, "[%s] error: sending commands to non-mux", id)
				}
				toProgram <- append(routing, m.Bytes())
			} else if ts := m.TerminalSize(); ts != nil {
				if err := terminal.SetSize(f, ts); err != nil {
					logf(out, "[%s] error: setting size: %s", id, err.Error())
				}
			} else {
				toProgram <- [][]byte{m.Bytes()}
			}

		case m, ok := <-fromProgram:
			if !ok || m == nil {
				goto done
			}

			if m.Logging() {
				toTerminal <- [][]byte{m.Bytes()}

				continue
			}

			if src.Buffered(m) {
				continue
			}

			if m.IsStarted() {
				nested++
				statusq <- &Status{n: 1}
			} else if m.IsStatus() {
				nested--
				statusq <- &Status{n: -1}
			}

			bs := append(src.Routing(), m.Bytes())
			/*
				logf(toTerminal, "mux sent {")
				for _, b := range bs {
					logf(toTerminal, "mux sent: %s", message.Raw(b))
				}
				logf(toTerminal, "}")
			*/

			toTerminal <- bs
		}
	}

done:
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
	rv := 0
	defer func() {
		if rv != 0 {
			os.Exit(rv)
		}
	}()

	request := false

	flag.Usage = func() {
		f := flag.CommandLine.Output()
		fmt.Fprintf(f, "%s\n\nUsage:\n", os.Args[0])
		fmt.Fprintf(f, "  %s [-l LABEL] COMMAND ARGUMENTS...\n", os.Args[0])
		fmt.Fprintf(f, "  %s -n COMMAND ARGUMENTS...\n\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.StringVar(&label, "l", label, "mux label (for debugging)")
	flag.BoolVar(&request, "n", request, "request new local session")
	flag.Parse()

	args, defaulted := config.Command()

	if request {
		os.Stdout.Write(message.Run(args, os.Environ()))

		return
	} else if defaulted && terminal.IsTTY() {
		flag.Usage()

		rv = 1

		return
	}

	done := make(chan struct{})
	fromServer := comms.Chunk(os.Stdin)
	id := ""
	nested := 0
	next := comms.Counter(1)
	routing := []*message.T{}
	status := (*Status)(nil)
	statusq := make(chan *Status, 1) // Pty ID + exit status.
	stream := map[string]chan *message.T{}
	toServer := comms.Write(os.Stdout, done)

	defer func() {
		if status != nil {
			rv = status.rv

			toServer <- [][]byte{
				message.Term(status.term),
				message.Pty(status.pty),
				message.Status(rv),
			}
		}

		close(toServer)
		<-done
	}()

	if terminal.IsTTY() {
		id = "0"

		c := make(chan *message.T)
		stream[id] = c

		restore, err := terminal.MakeRaw()
		if err != nil {
			println("failed to put terminal in raw mode")

			return
		}

		defer restore()

		go session(id, c, toServer, statusq)
		c <- message.Raw(message.Term(""))
		c <- message.Raw(message.Run(args, os.Environ()))
		c <- message.Raw(message.TerminalSize(terminal.GetSize()))
	}

	for {
		select {
		case m := <-fromServer:
			if m == nil {
				return
			}

			logf(toServer, "mux recv: %s", m)

			if m.Is(message.Command) {
				switch {
				case m.IsPty():
					if id == "" {
						id = m.Pty()
					} else {
						routing = append(routing, m)
					}

					continue

				case m.IsTerm():
					id = ""
					routing = []*message.T{m}

					continue

				case m.IsRun():
					if id == "" {
						id = <-next
						c := make(chan *message.T)

						stream[id] = c

						go session(id, c, toServer, statusq)
					}
				}
			}

			selected := stream[id]
			if selected == nil {
				continue
			}

			for _, b := range routing {
				selected <- b
			}

			selected <- m

		case s := <-statusq:
			if s.n != 0 {
				nested += s.n

				continue
			}

			close(stream[s.pty])
			delete(stream, s.pty)

			if s.pty == "0" {
				status = s
			} else {
				toServer <- [][]byte{
					message.Term(s.term),
					message.Pty(s.pty),
					message.Status(s.rv),
				}
			}

			if nested == 0 && len(stream) == 0 {
				return
			}
		}
	}
}
