// Released under an MIT license. See LICENSE.

// Package message encapsulates the units emitted by the lexer.
package message

func (m *message) Configuration() bool {
	// NOTE: This may be expanded to include other commands.
	return m.Is(Escape) && m.Command() == "set-window-size"
}

func (m *message) IsMux() bool {
	return m.Is(Escape) && m.Command() == "mux"
}

func (m *message) IsPty() bool {
	return m.Is(Escape) && m.Command() == "pty"
}

func (m *message) IsRun() bool {
	return m.Is(Escape) && m.Command() == "run"
}

func (m *message) IsStarted() bool {
	return m.Is(Escape) && m.Command() == "started"
}

func (m *message) IsStatus() bool {
	return m.Is(Escape) && m.Command() == "status"
}

func (m *message) IsTerm() bool {
	return m.Is(Escape) && m.Command() == "term"
}

func (m *message) Logging() bool {
	return m.Is(Escape) && m.Command() == "log"
}

func (m *message) Meta() bool {
	if !m.Is(Escape) {
		return false
	}

	cmd := m.Command()
	return cmd == "mux" || cmd == "run" || cmd == "status"
}

func (m *message) Routing() bool {
	if !m.Is(Escape) {
		return false
	}

	cmd := m.Command()
	return cmd == "pty" || cmd == "term"
}
