// Released under an MIT license. See LICENSE.

// Package message encapsulates the units emitted by the lexer.
package buffer

import (
	"sync"

	"github.com/michaelmacinnis/summit/pkg/message"
)

type Buffer struct {
	sync.RWMutex

	prefix []*message.T

	buffering bool
	completed bool

	buffer  [][]byte
	routing [][]byte
}

func New(prefix ...*message.T) *Buffer {
	return &Buffer{
		buffer:  bytes(prefix),
		prefix:  prefix,
		routing: bytes(prefix),
	}
}

func (b *Buffer) Message(m *message.T) bool {
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
			if len(b.prefix) > 0 && b.prefix[0].IsTerm() {
				b.buffer[0] = m.Bytes()
			}
		}
	} else {
		if !b.completed {
			b.completed = true

			b.routing = b.buffer
			b.buffer  = bytes(b.prefix)
		}

		b.buffering = false
	}

	return b.buffering
}

func (b *Buffer) Remove() {
	b.Lock()
	defer b.Unlock()

	sz := len(b.routing)
	if sz > 0 {
		b.routing = b.routing[:sz-1]
	}
}

func (b *Buffer) Routing() [][]byte {
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
