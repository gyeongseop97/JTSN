#include "textflag.h"

TEXT ·winPtr(SB), NOSPLIT, $0-16
	MOVQ value+0(FP), AX
	MOVQ AX, ret+8(FP)
	RET
