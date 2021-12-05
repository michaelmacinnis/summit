// Released under an MIT license. See LICENSE.

// Package message encapsulates the units emitted by the lexer.
package message

func (m *message) Logging() bool {
	return m.Is(Escape) && m.Command() == "log"
}

func (m *message) Routing() bool {
	if !m.Is(Escape) {
		return false
	}

	cmd := m.Command()
	return cmd == "pty" || cmd == "term"
}
