package config

import (
	"encoding/json"
	"flag"
	"os"
)

func Command() ([]string, bool) {
	args := flag.Args()
	if len(args) > 0 {
		return args, false
	}

	return []string{command}, true
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

func Parse() {
	flag.StringVar(&socket, "s", socket, "path to summit server socket")
	flag.Parse()
}

func Socket() string {
	return socket
}

var (
	command = Get("SUMMIT_COMMAND", Get("SHELL", "/bin/bash"))
	socket  = Get("SUMMIT_SOCKET", "/tmp/summit.socket")
)
