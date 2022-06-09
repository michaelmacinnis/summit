// Released under an MIT license. See LICENSE.

// Package message encapsulates the units emitted by the lexer.
package message

import (
	"fmt"
	"strconv"
	"unicode"
)

// Class is a message's type.
type Class rune

// Message classes.
const (
	Error Class = iota

	End Class = unicode.MaxRune + iota
	Escape
	Text
)

// String returns a string representation of Class. Useful for debugging.
func (c *Class) String() string {
	switch *c {
	case Error:
		return "error"
	case Escape:
		return "escape"
	case Text:
		return "text"
	}

	return strconv.QuoteRune(rune(*c))
}

// T (message) is a lexical item returned by the scanner.
type T struct {
	cls Class
	raw []byte
	kv  map[string]interface{}
}

type message = T

// Command creates a new Escape "command" message.
func Command(raw []byte) *message {
    c := &message{
        cls: Escape,
        raw: raw,
    }

    return c
}

// KV creates a new message from a map.
func KV(kv map[string]interface{}) *message {
	c := &message{
		cls: Escape,
		kv:  kv,
	}

	return c
}

func Log(format string, i ...interface{}) *message {
	return Command(command("log", fmt.Sprintf(format, i...)))
}

// New creates a new message of unparsed bytes.
func New(cls Class, raw []byte) *message {
	c := &message{
		cls: cls,
		raw: raw,
	}

	return c
}

// Bytes returns the message's raw bytes.
func (m *message) Bytes() []byte {
	if m.raw == nil && m.kv != nil && m.Is(Escape) {
		m.raw = Serialize(m.kv)
	}
	return m.raw
}

// Is returns true if the message t is any of the classes in cs.
func (m *message) Is(cs ...Class) bool {
	if m == nil {
		return false
	}

	for _, c := range cs {
		if m.cls == c {
			return true
		}
	}

	return false
}

// Parsed returns the message's parsed value.
func (m *message) Parsed() map[string]interface{} {
	if m.kv == nil && m.raw != nil && m.Is(Escape) {
		m.kv = Deserialize(m.raw)
	}
	return m.kv
}

// String returns the message's string representation. Useful for debugging.
func (m *message) String() string {
	return "(" + m.cls.String() + ": " + strconv.Quote(string(m.raw)) + ")"
}
