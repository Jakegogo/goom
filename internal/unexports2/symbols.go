package unexports2

import (
	"debug/gosym"
	"fmt"
	"strings"
)

var (
	symTable          *gosym.Table
	symTableLoadError error
)

func loadSymbolTable() (table *gosym.Table, err error) {
	if err = symTableLoadError; err != nil {
		return
	}

	if table = symTable; table != nil {
		return
	}

	table, err = osReadSymbolsFromExeFile()
	symTableLoadError = err
	if err != nil {
		symTable = nil
	} else if table == nil {
		err = fmt.Errorf("Unknown error: symbol table was nil")
	} else {
		symTable = table
	}

	symTable = table
	return
}

// GetFunctionSymbol returns the symbols for a given function.
func GetFunctionSymbol(function interface{}) (symbol *gosym.Func, err error) {
	var table *gosym.Table
	if table, err = GetSymbolTable(); err != nil {
		return
	}

	address, err := getFunctionAddress(function)
	if err != nil {
		return
	}
	symbol = table.PCToFunc(uint64(address))
	if symbol == nil {
		err = fmt.Errorf("Function symbol at %x not found", address)
	}
	return
}

func getFunctionSymbolByName(name string) (symbol *gosym.Func, err error) {
	var table *gosym.Table
	if table, err = GetSymbolTable(); err != nil {
		return
	}

	symbol = table.LookupFunc(name)
	if symbol == nil {
		err = fmt.Errorf("%v: function symbol not found", name)
	}
	return
}

func getVarSymbolByName(name string) (symbol *gosym.Sym, err error) {
	var table *gosym.Table
	if table, err = GetSymbolTable(); err != nil {
		return
	}

	symbol = lookupSym(table, name)
	if symbol == nil {
		err = fmt.Errorf("%v: variable symbol not found", name)
	}
	return
}

// LookupSym returns the text, data, or bss symbol with the given name,
// or nil if no such symbol is found.
func lookupSym(t *gosym.Table, name string) *gosym.Sym {
	// Stability note (darwin/arm64):
	// Mach-O symbol tables may include a leading '_' for data symbols depending on how the binary is produced.
	// The higher-level APIs (users) refer to Go symbol names without this underscore.
	//
	// If we require an exact match only, FindVarByName can fail spuriously and callers end up
	// constructing invalid pointers or panicking during unexported-var mocking.
	// TODO(austin) Maybe make a map
	for i := range t.Syms {
		s := &t.Syms[i]
		if s.Name == name || s.Name == "_"+name || strings.TrimPrefix(s.Name, "_") == name {
			return s
		}
	}
	return nil
}
