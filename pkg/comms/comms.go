// Released under an MIT license. See LICENSE.

package comms

import (
	"io"
	"strconv"

	"github.com/michaelmacinnis/summit/pkg/lexer"
	"github.com/michaelmacinnis/summit/pkg/message"
)

const blocksz = 65536

func Counter(initial uint64) chan string {
	next := make(chan string)

	go func() {
		i := initial

		for {
			next <- strconv.FormatUint(i, 10)
			i++
		}
	}()

	return next
}

func Read(r io.Reader) chan []byte {
	c := make(chan []byte)

	go Reader(r, func(b []byte) {
		if b != nil {
			c <- b
		} else {
			close(c)
		}
	})

	return c
}

func Reader(r io.Reader, f func(b []byte)) {
	for {
		b := make([]byte, blocksz)

		n, _ := r.Read(b)
		if n <= 0 {
			break
		}

		f(b[:n])
	}

	f(nil)
}

func Chunk(r io.Reader) chan *message.T {
	c := make(chan *message.T)

	l := lexer.New()

	go Reader(r, func(b []byte) {
		if b != nil {
			l.Scan(b)
			for t := l.Chunk(); t != nil; t = l.Chunk() {
				c <- t
			}
		} else {
			close(c)
		}
	})

	return c
}

func Write(wc io.WriteCloser, ds ...chan struct{}) chan [][]byte {
	c := make(chan [][]byte)

	go func() {
		defer wc.Close()

		for bs := range c {
			for _, b := range bs {
				if b != nil {
					_, err := wc.Write(b)
					if err != nil {
						println(err.Error())
					}
				}
			}
		}

		// Signal completion.
		for _, d := range ds {
			close(d)
		}
	}()

	return c
}
