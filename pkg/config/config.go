package config

import (
	"encoding/json"
	"flag"
	"os"
)

var (
	command = Get("SUMMIT_COMMAND", Get("SHELL", "/bin/bash"))
	socket  = Get("SUMMIT_SOCKET", "/tmp/summit.socket")
)

func Command() ([]string, bool) {
	args := flag.Args()
	if len(args) > 0 {
		return args, true
	}

	return []string{command}, false
}

func Env(j string) []string {
	env := []string{}

	if j != "" {
		err := json.Unmarshal([]byte(j), &env)
		if err != nil {
			env = os.Environ()
		}
	} else {
		env = os.Environ()
	}

	return env
}

func Get(k, dflt string) string {
	if v, found := os.LookupEnv(k); found {
		return v
	}

	return dflt
}

func Socket() string {
	return socket
}

func init() {
	flag.StringVar(&socket, "s", socket, "path to summit server socket")
}
