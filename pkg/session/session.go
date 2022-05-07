// Released under an MIT license. See LICENSE.

package session

import (
	"strings"

	"github.com/michaelmacinnis/summit/pkg/message"
)

func Path(offset int, routing [][]byte) (string, string) {
	path := make([]string, len(routing))
	n := 0

	term := ""
	for _, b := range routing {
		p := message.New(message.Escape, b)
		switch p.Command() {
		case "pty":
			path[n] = p.Pty()
			n++

		case "term":
			term = p.Terminal()
		}
	}

	n += offset
	if n > 0 {
		return term, strings.Join(path[:n], "-")
	}

	return term, ""
}
