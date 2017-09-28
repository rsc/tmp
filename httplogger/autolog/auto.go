// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package autolog

import (
	"net/http"
	"os"

	"rsc.io/tmp/httplogger"
)

func init() {
	if os.Getenv("HTTPLOG") == "1" {
		http.DefaultTransport = httplogger.New(http.DefaultTransport)
	}
}
