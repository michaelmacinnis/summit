// Released under an MIT license. See LICENSE.

// Package message encapsulates the units emitted by the lexer.
package buffer

import (
	"sync"

	"github.com/michaelmacinnis/summit/pkg/message"
)

type T struct {
	IgnoreBlankTerm bool

	sync.RWMutex

	prefix []*message.T

	buffering bool
	completed bool

	buffer  [][]byte
	routing [][]byte
}

type buffer = T

func New(prefix ...*message.T) *buffer {
	return &buffer{
		buffer:  bytes(prefix),
		prefix:  prefix,
		routing: bytes(prefix),
	}
}

func (b *buffer) Message(m *message.T) bool {
	b.Lock()
	defer b.Unlock()

	if m.Routing() {
		if !b.buffering {
			b.buffering = true
			b.completed = false
		}

		if m.IsPty() {
			b.buffer = append(b.buffer, m.Bytes())
		} else if m.IsTerm() {
			if m.Term() != "" || !b.IgnoreBlankTerm {
				if len(b.prefix) > 0 && b.prefix[0].IsTerm() {
					b.buffer[0] = m.Bytes()
				}
			}
		}

		return true
	}

	if !b.completed {
		b.routing = b.buffer
		b.buffer = bytes(b.prefix)
	}

	b.completed = !m.IsStatus()
	b.buffering = false

	return false
}

func (b *buffer) Remove() {
	b.Lock()
	defer b.Unlock()

	sz := len(b.routing)
	if sz > 0 {
		b.routing = b.routing[:sz-1]
	}
}

func (b *buffer) Routing() [][]byte {
	b.RLock()
	defer b.RUnlock()

	return b.routing
}

func bytes(ms []*message.T) [][]byte {
	bs := make([][]byte, 0, len(ms))

	for _, m := range ms {
		bs = append(bs, m.Bytes())
	}

	return bs
}
