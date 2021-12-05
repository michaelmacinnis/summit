// Released under an MIT license. See LICENSE.

package message

func Pty(pty string) []byte {
	return command("pty", pty)
}

func Run(cmd []string) []byte {
	return command("run", cmd)
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
        key: value,
    })
}
