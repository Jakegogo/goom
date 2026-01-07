//go:build go1.24 && arm64

package abijson

type S struct {
	A int            `json:"a"`
	B string         `json:"b"`
	M map[string]int `json:"m"`
	X interface{}    `json:"x"`
	P *int           `json:"p"`
}
