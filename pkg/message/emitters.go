// Released under an MIT license. See LICENSE.

package message

import (
	"github.com/michaelmacinnis/summit/pkg/terminal"
)

func Pty(pty string) []byte {
	return command("pty", pty)
}

func Run(cmd, env []string, ts *terminal.Size) []byte {
	return Serialize(map[string]interface{}{
		"cmd": "run",
		"env": env,
		"run": cmd,
		"ts":  ts,
	})
}

func Started() []byte {
	return Serialize(map[string]interface{}{
		"cmd": "started",
	})
}

func Status(status int) []byte {
	return command("status", status)
}

func Term(term string) []byte {
	return command("term", term)
}

func TerminalSize(ts *terminal.Size) []byte {
	return command("ts", ts)
}

func command(key string, value interface{}) []byte {
	return Serialize(map[string]interface{}{
		"cmd": key,
		key:   value,
	})
}
