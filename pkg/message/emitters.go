// Released under an MIT license. See LICENSE.

package message

import "os"

func Mux(mux int) []byte {
	return command("mux", mux)
}

func Pty(pty string) []byte {
	return command("pty", pty)
}

func Run(cmd []string) []byte {
	return Serialize(map[string]interface{}{
		"cmd": "run",
		"run": cmd,
		"env": os.Environ(),
	})
}

func Status(code int) []byte {
	return command("status", code)
}

func Terminal(terminal string) []byte {
	return command("term", terminal)
}

func command(key string, value interface{}) []byte {
	return Serialize(map[string]interface{}{
		"cmd": key,
		key:   value,
	})
}
