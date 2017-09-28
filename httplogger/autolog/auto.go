// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package autolog

import (
	"net/http"

	"rsc.io/tmp/httplogger"
)

func init() {
	http.DefaultTransport = httplogger.New(http.DefaultTransport)
}
