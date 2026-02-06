package shared

import (
	"encoding/json"
	"fmt"
)

// DumpPanic formats and prints a panic value as JSON.
func DumpPanic(v interface{}) {
	if v == nil {
		fmt.Printf("panic=%s\n", "null")
		return
	}
	var s string
	switch x := v.(type) {
	case string:
		s = x
	case error:
		s = x.Error()
	case interface{ String() string }:
		s = x.String()
	default:
		s = fmt.Sprintf("%T: %v", v, v)
	}
	b, _ := json.Marshal(s)
	fmt.Printf("panic=%s\n", string(b))
}
