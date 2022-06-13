// Released under an MIT license. See LICENSE.

// Package message encapsulates the units emitted by the lexer.
package message

import (
	"github.com/michaelmacinnis/summit/pkg/terminal"
)

func (m *message) Args() (args []string) {
	return m.strings("run")
}

func (m *message) Command() string {
	return field[string](m, "cmd")
}

func (m *message) Env() (env []string) {
	return m.strings("env")
}

func (m *message) Log() string {
	return field[string](m, "log")
}

func (m *message) Pty() string {
	return field[string](m, "pty")
}

func (m *message) Status() int {
	return int(field[float64](m, "status"))
}

func (m *message) Term() string {
	return field[string](m, "term")
}

func (m *message) TerminalSize() *terminal.Size {
	ts := field[map[string]interface{}](m, "ts")
	if ts == nil {
		return nil
	}

	return &terminal.Size{
		Rows: u16(ts["Rows"]),
		Cols: u16(ts["Cols"]),
		X:    u16(ts["X"]),
		Y:    u16(ts["Y"]),
	}
}

func (m *message) strings(k string) (elems []string) {
	a := field[[]any](m, k)

	elems = make([]string, len(a))

	for k, v := range a {
		elems[k] = v.(string)
	}

	return
}

func cast[T any](v any) T {
	var zero T

	if n, ok := v.(T); ok {
		return n
	}

	return zero
}

func field[T any](m *message, k string) T {
	return cast[T](m.Parsed()[k])
}

func u16(v any) uint16 {
	return uint16(cast[float64](v))
}
