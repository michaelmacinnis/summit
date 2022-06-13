// Released under an MIT license. See LICENSE.

// Package message encapsulates the units emitted by the lexer.
package message

func (m *message) IsPty() bool {
	return is(m, "pty")
}

func (m *message) IsRun() bool {
	return is(m, "run")
}

func (m *message) IsStarted() bool {
	return is(m, "started")
}

func (m *message) IsStatus() bool {
	return is(m, "status")
}

func (m *message) IsTerm() bool {
	return is(m, "term")
}

func (m *message) Logging() bool {
	return is(m, "log")
}

func (m *message) Routing() bool {
	return is(m, "pty", "term")
}

func is(m *message, cmds ...string) bool {
	s := m.Command()

	for _, cmd := range cmds {
		if s == cmd {
			return true
		}
	}

	return false
}
