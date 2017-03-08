// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gammabench

/*
go1.6 build -a -gcflags -S math 2>&1 | go2asm -s math.Lgamma | sed 's/math·Lgamma/·Lgamma16/' >lgamma16.s
go1.7 build -a -gcflags -S math 2>&1 | go2asm -s math.Lgamma | sed 's/math·Lgamma/·Lgamma17/' >lgamma17.s
go build -a -gcflags -S math 2>&1 | go2asm -s math.Lgamma | sed 's/math·Lgamma/·LgammaZZZ/' >lgammazzz.s
*/

func Lgamma16(x float64) (lgamma float64, sign int)
func Lgamma17(x float64) (lgamma float64, sign int)
func LgammaZZZ(x float64) (lgamma float64, sign int)
