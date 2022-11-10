package wasm

import (
	"fmt"
)

type CallFrame struct {
	Pc          uint64
	FunctionIdx uint32 // function index
}

var BreakpointFlag uint32 = 0

func (frame CallFrame) String() string {
	return fmt.Sprintf("Fn %d@%d", frame.FunctionIdx, frame.Pc)
}
