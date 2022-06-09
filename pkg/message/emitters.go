// Released under an MIT license. See LICENSE.

package message

import (
	"github.com/creack/pty"
)

func Mux(mux int) []byte {
	return command("mux", mux)
}

func Pty(pty string) []byte {
	return command("pty", pty)
}

func Run(cmd, env []string, ws *pty.Winsize) []byte {
	return Serialize(map[string]interface{}{
		"cmd": "run",
		"env": env,
		"run": cmd,
		"ws":  ws,
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

func WindowSize(ws *pty.Winsize) []byte {
	return command("ws", ws)
}

func command(key string, value interface{}) []byte {
	return Serialize(map[string]interface{}{
		"cmd": key,
		key:   value,
	})
}
