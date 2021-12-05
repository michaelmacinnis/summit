// Released under an MIT license. See LICENSE.

package errors

import (
	"errors"
	"fmt"
	"os"
	"sync"
)

var (
	As = errors.As
	Is = errors.Is
)

func AtExit(f func() error) {
	mutex.Lock()
	defer mutex.Unlock()

	deferred = append(deferred, f)
}

func Die(format string, a ...interface{}) {
	println(fmt.Sprintf(format, a...))

    Exit(1)

}

func Exit(code int) {
	mutex.Lock()
	defer mutex.Unlock()

	for _, f := range deferred {
		err := f()
		if err != nil {
			println(err.Error())
		}
	}

	os.Exit(code)
}

func On(err error) handler {
	if err != nil {
		return real{err}
	}

	return null{}
}

var (
	deferred = []func() error{}
	mutex = sync.Mutex{}
)

type handler interface {
	Die(format string, a ...interface{})
}

type null struct{}

func (n null) Die(format string, a ...interface{}) {}

type real struct{error}

func (r real) Die(format string, a ...interface{}) {
	if format != "" {
		format += ": "
	}
	format += "%v"

	Die(format, append(a, r)...)
}
