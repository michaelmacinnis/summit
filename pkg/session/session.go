// Released under an MIT license. See LICENSE.

package session

import (
	"fmt"
	"strings"
	"sync"

	"github.com/michaelmacinnis/summit/pkg/message"
)

var (
	mutex sync.Mutex
	nodes = map[string]map[string]string{}
	sessions = map[string]map[string]string{}
)

func Path(offset int, routing [][]byte) (string, string) {
    path := make([]string, len(routing))
    n := 0

    term := ""
    for _, b := range routing {
        p := message.New(message.Escape, b)
        switch p.Command() {
        case "pty":
            path[n] = p.Pty()
            n++

        case "term":
            term = p.Terminal()
        }
    }

    n += offset
    if n > 0 {
        return term, strings.Join(path[:n], "-")
    }

    return term, ""
}

func Remove(routing [][]byte) (bool, [][]byte) {
	term, path := Path(0, routing)
	println("Removing:", term, path)

	mutex.Lock()
	defer mutex.Unlock()

	paths := sessions[term]

	if _, found := paths[path]; found {
		delete(paths, path)
	}

	if len(paths) == 0 {
		delete(sessions, term)
	}

	bs := [][]byte{}

	waypoints := strings.Split("-" + path, "-")
	parent := strings.Join(waypoints[1:len(waypoints)-1], "-")
	child := strings.Join(waypoints[len(waypoints)-1:], "-")
	println("Deregistering", parent, "->", child)
	children := nodes[parent]
	delete(children, child)
	if len(children) == 0 {
		println("Send status to node", parent)
		for _, s := range waypoints[1:len(waypoints)-1] {
			bs = append(bs, message.Pty(s))
		}
	}
	println("nodes", fmt.Sprintf("%v", nodes))

	return len(paths) == 0, bs
}

func Track(routing [][]byte) bool {
	term, path := Path(0, routing)

	println("Tracking:", term, path)

	mutex.Lock()
	defer mutex.Unlock()

	paths := sessions[term]
	if paths == nil {
		paths = map[string]string{}
		sessions[term] = paths
	}

	if _, found := paths[path]; !found {
		paths[path] = term

		waypoints := strings.Split("-" + path, "-")
		for i := 0; i < len(waypoints) - 1; i++ {
			parent := strings.Join(waypoints[1:i+1], "-")
			child := strings.Join(waypoints[i+1:], "-")

			println("Registering", parent, "->", child)
			children := nodes[parent]
			if children == nil {
				children = map[string]string{}
				nodes[parent] = children
			}
			children[child] = term
		}
	}

	return true
}

func Valid(routing [][]byte) [][]byte {
	term, path := Path(0, routing)

	// TODO: Validate that path is valid.
	println("Current:", term, path)

	return routing
}
