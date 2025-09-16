// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

TEXT ·now(SB),NOSPLIT,$0
	MRS	CNTVCT_EL0, R0
	MOVD	R0, ret+0(FP)
	RET
