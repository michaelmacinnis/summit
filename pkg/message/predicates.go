// Released under an MIT license. See LICENSE.

// Package message encapsulates the units emitted by the lexer.
package message

func (m *message) Configuration() bool {
	// NOTE: This may be expanded to include other commands.
	return m.Is(Command) && m.Command() == "ts"
}

func (m *message) IsPty() bool {
	return m.Is(Command) && m.Command() == "pty"
}

func (m *message) IsRun() bool {
	return m.Is(Command) && m.Command() == "run"
}

func (m *message) IsStarted() bool {
	return m.Is(Command) && m.Command() == "started"
}

func (m *message) IsStatus() bool {
	return m.Is(Command) && m.Command() == "status"
}

func (m *message) IsTerm() bool {
	return m.Is(Command) && m.Command() == "term"
}

func (m *message) Logging() bool {
	return m.Is(Command) && m.Command() == "log"
}

func (m *message) Meta() bool {
	if !m.Is(Command) {
		return false
	}

	cmd := m.Command()
	return cmd == "run" || cmd == "started" || cmd == "status"
}

func (m *message) Routing() bool {
	if !m.Is(Command) {
		return false
	}

	cmd := m.Command()
	return cmd == "pty" || cmd == "term"
}
