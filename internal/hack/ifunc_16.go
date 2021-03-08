// +build !go1.17,go1.16

package hack

import (
	// nolint
	"unsafe"
	_ "unsafe"
)

// Firstmoduledata TODO 兼容不同go版本
//go:linkname Firstmoduledata runtime.firstmoduledata
var Firstmoduledata Moduledata

//Moduledata Moduledata
type Moduledata struct {
	pcHeader     uintptr
	funcnametab  []byte
	cutab        []uint32
	filetab      []byte
	pctab        []byte
	Pclntable    []byte
	Ftab         []Functab
	findfunctab  uintptr
	minpc, maxpc uintptr

	text, etext           uintptr
	noptrdata, enoptrdata uintptr
	data, edata           uintptr
	bss, ebss             uintptr
	noptrbss, enoptrbss   uintptr
	end, gcdata, gcbss    uintptr
	types, etypes         uintptr

	textsectmap []textsect
	typelinks   []int32 // offsets from types
	itablinks   []*uintptr

	ptab []interface{}

	pluginpath string
	pkghashes  []interface{}

	modulename   string
	modulehashes []interface{}

	hasmain uint8 // 1 if module contains the main function, 0 otherwise

	gcdatamask, gcbssmask Bitvector

	typemap map[typeOff]*interface{} // offset to *_rtype in previous module

	bad bool // module failed to load and should be ignored

	Next *Moduledata
}

//Functab Functab
type Functab struct {
	Entry   uintptr
	Funcoff uintptr
}

// nolint
type textsect struct {
	// nolint
	vaddr    uintptr // prelinked section vaddr
	length   uintptr // section length
	baseaddr uintptr // relocated section address
}

//Bitvector Bitvector
type Bitvector struct {
	// nolint
	n int32 // # of bits
	// nolint
	bytedata *uint8
}

// nolint
type typeOff int32 // offset to an *rtype

// Convenience struct for modifying the underlying code pointer of a function
// value. The actual struct has other values, but always starts with a code
// pointer.
// TODO 不同go版本兼容
type Func struct {
	CodePtr uintptr
}

// TODO 不同go版本兼容
//Value reflect.Value
type Value struct {
	Typ  *uintptr
	Ptr  unsafe.Pointer
	Flag uintptr
}
