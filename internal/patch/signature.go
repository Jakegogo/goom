package patch

import (
	"fmt"
	"reflect"
)

// SignatureEquals 检测两个函数类型的参数的内存区段是否一致
func SignatureEquals(typeA, typeB reflect.Type) bool {
	// 检测参数对齐
	if typeA.NumIn() != typeB.NumIn() {
		panic(fmt.Sprintf("func signature mismatch, args len must:%d, actual:%d",
			typeA.NumIn(), typeB.NumIn()))
	}
	if typeA.NumOut() != typeB.NumOut() {
		panic(fmt.Sprintf("func signature mismatch, returns len must:%d, actual:%d",
			typeA.NumOut(), typeB.NumOut()))
	}
	for i := 0; i < typeA.NumIn(); i++ {
		if typeA.In(i).Size() != typeB.In(i).Size() {
			panic(fmt.Sprintf("func signature mismatch, args %d's size must:%d, actual:%d",
				i, typeA.In(i).Size(), typeB.In(i).Size()))
		}
	}
	for i := 0; i < typeA.NumOut(); i++ {
		if typeA.Out(i).Size() != typeB.Out(i).Size() {
			panic(fmt.Sprintf("func signature mismatch, returns %d's size must:%d, actual:%d",
				i, typeA.Out(i).Size(), typeB.Out(i).Size()))
		}
	}
	return true
}
