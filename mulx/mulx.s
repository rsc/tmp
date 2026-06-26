#include "textflag.h"

TEXT ·f(SB), NOSPLIT, $0
	MOVQ $-1, R8
	MOVQ $-1, R9
	MOVQ $1, R10
	MOVQ $1, DX
	ADDQ R8, R9
	MULXQ R10, R10, R11
	MOVQ $0, AX
	ADCQ AX, AX
	MOVQ AX, ret+0(FP)
	RET

// func cpuid(eaxArg, ecxArg uint32) (eax, ebx, ecx, edx uint32)
TEXT ·cpuid(SB), NOSPLIT, $0-24
	MOVL eaxArg+0(FP), AX
	MOVL ecxArg+4(FP), CX
	CPUID
	MOVL AX, eax+8(FP)
	MOVL BX, ebx+12(FP)
	MOVL CX, ecx+16(FP)
	MOVL DX, edx+20(FP)
	RET
