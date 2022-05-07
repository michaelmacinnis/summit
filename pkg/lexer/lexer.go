// Released under an MIT license. See LICENSE.

// Package lexer provides a lexical scanner for the summit framework.
//
// The summit lexer adapts the state function used by Go's text/template
// lexer and described in detail in Rob Pike's talk "Lexical Scanning in
// Go". See https://talks.golang.org/2011/lex.slide for more information.
package lexer

import (
	"bytes"
	"unicode/utf8"

	"github.com/michaelmacinnis/summit/pkg/message"
)

// T holds the state of the scanner.
type T struct {
	expected []string // Completion candidates.

	bytes []byte   // Buffer being scanned.
	first int      // Index of the current message's first byte.
	index int      // Index of the current byte.
	queue [][]byte // Buffers waiting to be scanned.
	state action   // Current action.

	messages chan *message.T
}

const qsize = 2

// New creates a new lexer/scanner.
func New() *T {
	return &T{state: text}
}

// Scan passes a text buffer to the lexer for scanning.
// If a buffer is currently being scanned, the new buffer will
// be appended to the list of buffers waiting to be scanned.
func (l *T) Scan(text []byte) {
	l.queue = append(l.queue, text)
}

// Chunk returns the next scanned message, or nil if no message is available.
func (l *T) Chunk() *message.T {
	for {
		l.gather()

		if len(l.bytes) == 0 {
			return nil
		}

		select {
		case t := <-l.messages:
			return t
		default:
			state := l.state(l)
			if state != nil {
				l.state = state
			} else {
				close(l.messages)
			}
		}
	}
}

type action func(*T) action

const eof = -1

func (l *T) accept(r rune, w int) {
	l.index += w
}

func (l *T) emit(c message.Class, v []byte) {
	if len(v) == 0 {
		return
	}

	if c == message.Escape && len(v) == 8 {
		c = message.End
	}

	l.messages <- message.New(c, v)
	l.skip()
}

func (l *T) gather() {
	if len(l.queue) == 0 {
		return
	}

	length := len(l.bytes)
	bytes := bytes.Join(l.queue, nil)

	if length > 0 && l.first < length {
		// Prepend leftover to new bytes.
		bytes = append(l.bytes[l.first:], bytes...)
	}

	l.queue = nil
	l.bytes = bytes
	l.index -= l.first
	l.first = 0
	l.messages = make(chan *message.T, qsize)
}

func (l *T) next() rune {
	r, w := l.peek()
	l.accept(r, w)

	return r
}

func (l *T) peek() (rune, int) {
	r, w := rune(eof), 0
	if l.index < len(l.bytes) {
		r, w = utf8.DecodeRune(l.bytes[l.index:])
	}

	return r, w
}

func (l *T) skip() {
	l.first = l.index
}

func (l *T) text() []byte {
	return l.bytes[l.first:l.index]
}

// T states.

func afterCloseBrace(l *T) action {
	return on(l, '-', afterCloseBraceDash)
}

func afterCloseBraceDash(l *T) action {
	return on(l, 0x1b, afterCloseBraceDashEscape)
}

func afterCloseBraceDashEscape(l *T) action {
	r, w := l.peek()

	switch r {
	case eof:
		return nil
	case '\\':
		l.accept(r, w)
		l.emit(message.Escape, l.text())
	}

	return text
}

func afterEscape(l *T) action {
	// println("afterEscape")
	return on(l, '^', afterEscapeCaret)
}

func afterEscapeCaret(l *T) action {
	return on(l, '-', afterEscapeCaretDash)
}

func afterEscapeCaretDash(l *T) action {
	return on(l, '{', base64UntilCloseBrace)
}

func base64Char(r rune) bool {
	set := "+/0123456789=ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	for _, v := range set {
		if r == v {
			return true
		}
	}

	return false
}

func base64UntilCloseBrace(l *T) action {
	for {
		r, w := l.peek()

		switch r {
		case eof:
			return nil
		case '}':
			l.accept(r, w)
			return afterCloseBrace
		default:
			if !base64Char(r) {
				return text
			}
			l.accept(r, w)
		}
	}
}

func on(l *T, expected rune, perform action) action {
	actual, width := l.peek()

	switch actual {
	case eof:
		return nil
	case expected:
		l.accept(actual, width)
		return perform
	default:
		return text
	}
}

func text(l *T) action {
	for {
		r, w := l.peek()

		switch r {
		case eof:
			l.emit(message.Text, l.text())
			return nil
		case 0x1b:
			l.emit(message.Text, l.text())
			l.accept(r, w)
			return afterEscape
		default: // Continue and get next character.
			l.accept(r, w)
		}
	}
}
