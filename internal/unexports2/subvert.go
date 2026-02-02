// Package subvert provides functions to subvert go's type & memory protections,
// and expose unexported values & functions.
//
// This is not a power to be taken lightly! It's expected that you're fully
// versed in how the go type system works, and why there are protections and
// restrictions in the first place. Using this package incorrectly will quickly
// lead to undefined behavior and bizarre crashes, even segfaults or nuclear
// missile launches.

// YOU HAVE BEEN WARNED!
package unexports2

import (
	"debug/gosym"
)

// ExposeFunction exposes a function or method, allowing you to bypass export
// restrictions. It looks for the symbol specified by funcSymName and returns a
// function with its implementation, or nil if the symbol wasn't found.
//
// funcSymName must be the exact symbol name from the binary. Use AllFunctions()
// to find it. If your program doesn't have any references to a function, it
// will be omitted from the binary during compilation. You can prevent this by
// saving a reference to it somewhere, or calling a function that indirectly
// references it.
//
// templateFunc MUST have the correct function type, or else undefined behavior
// will result!
//
// Example:
//
//	exposed := ExposeFunction("reflect.methodName", (func() string)(nil))
//	if exposed != nil {
//	    f := exposed.(func() string)
//	    fmt.Printf("Result of reflect.methodName: %v\n", f())
//	}
func ExposeFunction(funcSymName string, templateFunc interface{}) (function interface{}, err error) {
	// Ensure funcAlignment is initialized (symbol-table entry addresses are not the same as runtime memory addresses).
	// Without this, we may build a function value with an invalid code pointer and crash on call.
	initAlignment.Do(initAlignmentFunc)
	fn, err := getFunctionSymbolByName(funcSymName)
	if err != nil {
		return
	}
	return newFunctionWithImplementation(templateFunc, uintptr(fn.Entry)+funcAlignment)
}

// GetSymbolTable loads (if necessary) and returns the symbol table for this process
func GetSymbolTable() (*gosym.Table, error) {
	if symTable == nil && symTableLoadError == nil {
		symTable, symTableLoadError = loadSymbolTable()
	}

	return symTable, symTableLoadError
}

// AllFunctions returns the name of every function that has been compiled
// into the current binary. Use it as a debug helper to see if a function
// has been compiled in or not.
func AllFunctions() (functions map[string]bool, err error) {
	var table *gosym.Table
	if table, err = GetSymbolTable(); err != nil {
		return
	}

	functions = make(map[string]bool)
	for _, function := range table.Funcs {
		functions[function.Name] = true
	}
	return
}
