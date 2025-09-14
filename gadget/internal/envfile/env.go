// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package envfile

import (
	"os"
	"strings"
	"strconv"
)

// Load loads env values from the named file.
// The file's KEY=VAL pairs must each be on a single line.
// It is permitted to put "" or '' around KEY or VAL, in which
// Go's unquote is used.
// Spaces around KEY or VAL are ignored.
func Load(file string) (map[string]string, error) {
	m := make(map[string]string)
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	for line := range strings.Lines(string(data)) {
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, ok = unquote(key)
		if !ok {
			continue
		}
		val, ok = unquote(val)
		if !ok {
			continue
		}
		m[key] = val
	}
	return m, nil
}

func unquote(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, `"`) || strings.HasPrefix(s, `'`) {
		var err error
		if s, err = strconv.Unquote(s); err != nil {
			return "", false
		}
	}
	return s, true
}
