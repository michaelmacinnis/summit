// Released under an MIT license. See LICENSE.

// Package message encapsulates the units emitted by the lexer.
package message

import (
	"github.com/creack/pty"
)

func (m *message) Args() []string {
	var args []string
    if a, ok := value(m.Parsed(), "run").([]interface{}); ok {
        args = make([]string, len(a))
        for k, v := range a {
            args[k] = v.(string)
        }
    }

	return args
}

func (m *message) Command() string {
	return str(m.Parsed(), "cmd")
}

func (m *message) Log() string {
	return str(m.Parsed(), "log")
}

func (m *message) Pty() string {
	return str(m.Parsed(), "pty")
}

func (m *message) Status() int {
	return int(num(m.Parsed(), "status"))
}

func (m *message) Terminal() string {
	return str(m.Parsed(), "term")
}

func (m *message) WindowSize() *pty.Winsize {
    ws := sub(m.Parsed(), "ws")
    if ws == nil {
        return nil
    }

    return &pty.Winsize{
        Rows: u16(ws, "Rows"),
        Cols: u16(ws, "Cols"),
        X:    u16(ws, "X"),
        Y:    u16(ws, "Y"),
    }
}

func num(m map[string]interface{}, k string) float64 {
    v := value(m, k)
    if n, ok := v.(float64); ok {
        return n
    }

    return 0
}

func str(m map[string]interface{}, k string) string {
    if s, ok := value(m, k).(string); ok {
        return s
    }

    return ""
}

func sub(m map[string]interface{}, k string) map[string]interface{} {
    v := value(m, k)
    if m, ok := v.(map[string]interface{}); ok {
        return m
    }

    return nil
}

func u16(m map[string]interface{}, k string) uint16 {
    return uint16(num(m, k))
}

func value(m map[string]interface{}, k string) interface{} {
    if v, ok := m[k]; ok {
        return v
    }

    return nil
}
