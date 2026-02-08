package arg

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"unsafe"

	"github.com/tencent/goom/internal/hack"
	"github.com/tencent/goom/internal/iface"
)

// I2V []interface convert to []reflect.Value
func I2V(objs []interface{}, types []reflect.Type, isVariadic bool) ([]reflect.Value, error) {
	if isVariadic {
		if len(objs) < len(types)-1 {
			return nil, fmt.Errorf("the number of args does not match, required: %d, actual: %d", len(types), len(objs))
		}
	} else {
		if len(objs) != len(types) {
			return nil, fmt.Errorf("the number of args does not match, required: %d, actual: %d", len(types), len(objs))
		}
	}

	values := make([]reflect.Value, len(objs))
	var e error
	var typ reflect.Type
	for i, a := range objs {
		if i < len(types)-1 {
			typ = types[i]
		} else {
			typ = types[len(types)-1]
			// 兼容可变参数
			if isVariadic {
				typ = typ.Elem()
			}
		}
		values[i], e = toValue(a, typ, isVariadic)
		if e != nil {
			return nil, e
		}
	}

	return values, nil
}

// toValue 转化为数值
func toValue(r interface{}, out reflect.Type, isVariadic bool) (reflect.Value, error) {
	v := reflect.ValueOf(r)
	outKind := out.Kind()
	isNil := r == nil
	allowNilOut := outKind == reflect.Interface || outKind == reflect.Ptr || outKind == reflect.Slice ||
		outKind == reflect.Map || outKind == reflect.Array || outKind == reflect.Chan
	var vType reflect.Type
	if !isNil {
		vType = v.Type()
	}

	if !isNil && vType != out && (outKind == reflect.Struct || outKind == reflect.Ptr) {
		if vType.Size() != out.Size() {
			return reflect.Value{}, fmt.Errorf("the type of the args does not match, required: %s, actual: %v", out, vType)
		}
		// 类型强制转换,适用于结构体 fake 场景
		v = cast(v, out)
	}

	switch {
	case isNil && allowNilOut:
		v = reflect.Zero(out)
	case !isNil && vType.Kind() == reflect.Ptr && vType == reflect.TypeOf(&iface.IContext{}):
		panic("goom not support Return() API when returns mocked interface type, please use Apply() API instead.")
	case !isNil && outKind == reflect.Interface:
		ptr := reflect.New(out)
		ptr.Elem().Set(v)
		v = ptr.Elem()
	case !isNil && vType.Size() != out.Size():
		return reflect.Value{}, fmt.Errorf("the type of the args does not match, required: %s, actual: %v", out, vType)
	}
	return v, nil
}

// cast 将reflect.Value类型强制转换为执行type类型的reflect.Value
func cast(v reflect.Value, typ reflect.Type) reflect.Value {
	originV := (*hack.Value)(unsafe.Pointer(&v))
	newV := reflect.NewAt(typ, originV.Ptr).Elem()
	newVHack := (*hack.Value)(unsafe.Pointer(&newV))
	v = *(*reflect.Value)(unsafe.Pointer(&hack.Value{
		Typ:  newVHack.Typ,
		Ptr:  originV.Ptr,
		Flag: originV.Flag,
	}))

	return v
}

// V2I []reflect.Value convert to []interface 将reflect.Value元素类型的数组，转换成interface元素类型的数组
func V2I(params []reflect.Value, types []reflect.Type) []interface{} {
	values := make([]interface{}, len(params))
	for i, a := range params {
		if (types[i].Kind() == reflect.Interface || types[i].Kind() == reflect.Ptr) && isZero(a) {
			values[i] = nil
		} else {
			values[i] = a.Interface()
		}
	}
	return values
}

// SprintV []reflect.Value print to string 将[]reflect.Value参数列表转换为string用于打印输出
func SprintV(params []reflect.Value) string {
	s := make([]string, 0, len(params))
	for _, a := range params {
		if (a.Kind() == reflect.Interface || a.Kind() == reflect.Ptr) && isZero(a) {
			s = append(s, "nil")
		} else {
			s = append(s, fmt.Sprintf("%v", a.Interface()))
		}
	}
	return strings.Join(s, ",")
}

// ToExpr 将[]interface{}参数转换成[]Expr
func ToExpr(args []interface{}, types []reflect.Type, isVariadic bool) ([]Expr, error) {
	if isVariadic {
		if len(args) < len(types)-1 {
			return nil, fmt.Errorf("the number of args does not match, required: %d, actual: %d", len(types), len(args))
		}
	} else {
		if len(args) != len(types) {
			return nil, fmt.Errorf("the number of args does not match, required: %d, actual: %d", len(types), len(args))
		}
	}
	// TODO results check
	expressions := make([]Expr, len(args))
	for i, a := range args {
		var typ reflect.Type
		if i < len(types)-1 {
			typ = types[i]
		} else {
			typ = types[len(types)-1]
		}

		if expr, ok := a.(Expr); ok {
			expressions[i] = expr
		} else {
			// 兼容可变参数
			if isVariadic {
				typ = typ.Elem()
			}
			// 默认使用 equals 表达式
			expressions[i] = Equals(a)
		}

		if err := expressions[i].Resolve([]reflect.Type{typ}, isVariadic); err != nil {
			return nil, err
		}
	}
	return expressions, nil
}

// isZero reports whether v is the zero value for its type.
// It panics if the argument is invalid.
func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return math.Float64bits(v.Float()) == 0
	case reflect.Complex64, reflect.Complex128:
		c := v.Complex()
		return math.Float64bits(real(c)) == 0 && math.Float64bits(imag(c)) == 0
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if !isZero(v.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice, reflect.UnsafePointer:
		return v.IsNil()
	case reflect.String:
		return v.Len() == 0
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !isZero(v.Field(i)) {
				return false
			}
		}
		return true
	default:
		return true
	}
}
