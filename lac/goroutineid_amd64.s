#include "go_asm.h"
#include "textflag.h"
#include "../../src/runtime/go_tls.h"


TEXT ·goRoutinePtr(SB),NOSPLIT,$0-8
	get_tls(CX)
	MOVQ g(CX), AX
	MOVQ AX, goid+0(FP)
	RET
