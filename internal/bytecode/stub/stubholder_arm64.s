#include "textflag.h"

#define NOP WORD $0x1f2003d5
#define NOP8 NOP; NOP; NOP; NOP; NOP; NOP; NOP; NOP;
#define NOP64 NOP8; NOP8; NOP8; NOP8; NOP8; NOP8; NOP8; NOP8;
#define NOP256 NOP64; NOP64; NOP64; NOP64;
#define NOP512 NOP64; NOP64; NOP64; NOP64; NOP64; NOP64; NOP64; NOP64;
#define NOP4096 NOP512; NOP512; NOP512; NOP512; NOP512; NOP512; NOP512; NOP512;
#define NOP16384 NOP4096; NOP4096; NOP4096; NOP4096;

//func ICachePaddingLeft()
TEXT ·ICachePaddingLeft(SB), $0-0
    NOP4096
    NOP256
    RET

//func ClearICache()
TEXT ·ClearICache(SB), $0-0
    NOP512
    RET

//func Placeholder()
TEXT ·Placeholder(SB), $0-0
    NOP16384
    NOP4096
    RET
