// Released under an MIT license. See LICENSE.

package message

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
)

var (
	CRLF = []byte{13, 10}
	EOT  = []byte{4}
)

func Deserialize(b []byte) map[string]interface{} {
	if !bytes.HasPrefix(b, pm) {
		return nil
	}

	n := bytes.Index(b, st)
	if n == -1 {
		return nil
	}

	d := json.NewDecoder(base64.NewDecoder(b64, bytes.NewBuffer(b[lpm:n])))

	m := map[string]interface{}{}
	err := d.Decode(&m)
	if err != nil {
		println(err.Error())
		return nil
	}

	return m
}

func Serialize(m map[string]interface{}) []byte {
	b := &bytes.Buffer{}

	inner := base64.NewEncoder(b64, b)
	e := json.NewEncoder(inner)

	err := e.Encode(m)
	if err != nil {
		return nil
	}

	// Needed to flush base64 encoded data to buffer.
	inner.Close()

	r := b.Bytes()
	n := len(r)

	s := make([]byte, n+lpm+len(st))

	copy(s[:lpm], pm)
	n += lpm

	copy(s[lpm:n], r)
	copy(s[n:], st)

	return s
}

var (
	b64 = base64.StdEncoding
	lpm = len(pm)

	// Control messages have the form: ESC^-{Base 64 encoded JSON}-ESC\.
	pm = []byte{0x1b, '^', '-', '{'}  // ESC ^ (PM) then "-{".
	st = []byte{'}', '-', 0x1b, '\\'} // "}-" then ESC \ (ST).
)
