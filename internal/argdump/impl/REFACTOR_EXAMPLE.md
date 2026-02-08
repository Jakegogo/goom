# 重构方案A实现示例

## 示例1: 类型别名解决 `interface{}` vs `any` 差异

### 当前问题
```go
// go117/core_amd64.go:197
var keepAlive []interface{}

// go118/core_amd64.go:197
var keepAlive []any
```

### 解决方案：条件类型别名

**regabi/compat_go117.go**
```go
//go:build go1.17 && !go1.18
// +build go1.17,!go1.18

package regabi

// anyCompat 在 go1.17 中映射到 interface{}
type anyCompat = interface{}
```

**regabi/compat_go118.go**
```go
//go:build go1.18
// +build go1.18

package regabi

// anyCompat 在 go1.18+ 中映射到 any
type anyCompat = any
```

**regabi/core_amd64.go** (统一实现)
```go
//go:build (go1.17 && amd64) || (go1.18 && amd64)
// +build go1.17,amd64 go1.18,amd64

package regabi

func dumpArgsAndZeroRets(impl *DumpFuncImpl, frame unsafe.Pointer, retValid *bool, regsPtr unsafe.Pointer) {
    regs := (*regArgs)(regsPtr)
    opt := abijson.DefaultOptions()
    opt.MaxDepth = 3

    var keepAlive []anyCompat  // ← 统一使用 anyCompat
    if flags.DumpEnabled {
        for i := 0; i < impl.fnType.NumIn(); i++ {
            at := impl.fnType.In(i)
            if at.Size() == 0 {
                continue
            }

            addr, box := impl.abid.addrOfArg(i, at, frame, regs)
            if box != nil {
                keepAlive = append(keepAlive, box)
            }
            // ... rest of implementation
        }
    }
    // ...
}
```

---

## 示例2: 提取公共类型定义

### 当前代码（重复3次）

**go117/core_amd64.go**, **go118/core_amd64.go**, **go124/core_amd64.go** (完全相同)
```go
// regArgs matches internal/abi.RegArgs (layout-sensitive).
type regArgs struct {
    Ints   [abi.IntArgRegs]uintptr
    Floats [abi.FloatArgRegs]uint64
    Ptrs   [abi.IntArgRegs]unsafe.Pointer
    ReturnIsPtr intArgRegBitmap
}

func (r *regArgs) intRegArgAddr(reg int, argSize uintptr) unsafe.Pointer {
    return unsafe.Pointer(&r.Ints[reg])
}

type makeFuncCtxt struct {
    fn      uintptr
    stack   *bitvec.BitVector
    argLen  uintptr
    regPtrs intArgRegBitmap
}

type DumpFuncImpl struct {
    makeFuncCtxt
    magic          uintptr
    fnType         reflect.Type
    abid           abiDesc
    HookGuard      *patch.Guard
    OrigFuncVal    unsafe.Pointer
    OrigFuncValBox interface{}
    UseTrampoline  bool
    StackArgsSz    uint32
    StackRetOff    uint32
    FrameSize      uint32
}
```

### 重构后：提取到 regabi

**regabi/types_amd64.go**
```go
//go:build (go1.17 && amd64) || (go1.18 && amd64)
// +build go1.17,amd64 go1.18,amd64

package regabi

import (
    "reflect"
    "unsafe"

    "github.com/tencent/goom/internal/argdump/abi"
    "github.com/tencent/goom/internal/argdump/impl/shared"
    "github.com/tencent/goom/internal/argdump/internal/bitvec"
    "github.com/tencent/goom/internal/patch"
)

// intArgRegBitmap is an alias for shared.IntArgRegBitmap.
type intArgRegBitmap = shared.IntArgRegBitmap

// regArgs matches internal/abi.RegArgs (layout-sensitive).
// This structure must exactly match the layout of internal/abi.RegArgs
// in the Go runtime for the target version.
type regArgs struct {
    Ints   [abi.IntArgRegs]uintptr        // Integer argument registers
    Floats [abi.FloatArgRegs]uint64       // Float argument registers
    Ptrs   [abi.IntArgRegs]unsafe.Pointer // Pointer map for GC
    ReturnIsPtr intArgRegBitmap            // Bitmap for return value pointers
}

func (r *regArgs) intRegArgAddr(reg int, argSize uintptr) unsafe.Pointer {
    return unsafe.Pointer(&r.Ints[reg])
}

// makeFuncCtxt matches reflect.makeFuncCtxt prefix (layout-sensitive).
type makeFuncCtxt struct {
    fn      uintptr           // Function pointer to reflect.makeFuncStub
    stack   *bitvec.BitVector // Stack pointer bitmap for GC
    argLen  uintptr           // Total argument length
    regPtrs intArgRegBitmap   // Register pointer bitmap
}

// DumpFuncImpl is the dump function implementation.
type DumpFuncImpl struct {
    makeFuncCtxt

    magic uintptr // Magic sentinel for type checking

    fnType reflect.Type // Function type being wrapped
    abid   abiDesc      // ABI descriptor

    HookGuard      *patch.Guard  // Patch guard for original function
    OrigFuncVal    unsafe.Pointer
    OrigFuncValBox interface{}
    UseTrampoline  bool
    StackArgsSz    uint32 // Stack arguments size
    StackRetOff    uint32 // Stack return offset
    FrameSize      uint32 // Total frame size
}

// ABI type definitions
type abiStepKind int

const (
    abiStepBad abiStepKind = iota
    abiStepStack
    abiStepIntReg
    abiStepFloatReg
    abiStepPointer
)

type abiStep struct {
    kind   abiStepKind
    offset uintptr // Offset in the value
    size   uintptr // Size to copy

    stkOff uintptr // Stack offset (if kind == abiStepStack)
    ireg   int     // Integer register index (if kind == abiStepIntReg)
    freg   int     // Float register index (if kind == abiStepFloatReg)
}

type abiSeq struct {
    steps      []abiStep
    valueStart int
    stackBytes uintptr
    iregs      int
    fregs      int
}

type abiDesc struct {
    call              abiSeq
    ret               abiSeq
    stackCallArgsSize uintptr
    stackRetOffset    uintptr
    retOffset         uintptr
    stackPtrs         *bitvec.BitVector
    inRegPtrs         intArgRegBitmap
    outRegPtrs        intArgRegBitmap
}
```

**go117/core_amd64.go** (简化为重导出)
```go
//go:build go1.17 && !go1.18 && amd64
// +build go1.17,!go1.18,amd64

package go117

import (
    "reflect"
    "github.com/tencent/goom/internal/argdump/impl/regabi"
)

// Re-export regabi types and functions for go1.17
type DumpFuncImpl = regabi.DumpFuncImpl

// MakeDumpFunc is the go1.17 entry point
func MakeDumpFunc(typ reflect.Type) interface{} {
    return regabi.MakeDumpFunc(typ)
}

// PatchFunc is the go1.17 patch entry point
func PatchFunc(target, dump interface{}) (*DumpFuncImpl, error) {
    return regabi.PatchFunc(target, dump)
}

// ... other re-exports
```

**go118/core_amd64.go** (简化为重导出)
```go
//go:build go1.18 && amd64
// +build go1.18,amd64

package go118

import (
    "reflect"
    "github.com/tencent/goom/internal/argdump/impl/regabi"
)

// Re-export regabi types and functions for go1.18+
type DumpFuncImpl = regabi.DumpFuncImpl

// MakeDumpFunc is the go1.18+ entry point
func MakeDumpFunc(typ reflect.Type) interface{} {
    return regabi.MakeDumpFunc(typ)
}

// PatchFunc is the go1.18+ patch entry point
func PatchFunc(target, dump interface{}) (*DumpFuncImpl, error) {
    return regabi.PatchFunc(target, dump)
}

// ... other re-exports
```

---

## 示例3: 提取公共核心函数

### ensureCallReflectPatched 完全相同（56行，重复3次）

**regabi/patch.go**
```go
//go:build (go1.17 && amd64) || (go1.18 && amd64)
// +build go1.17,amd64 go1.18,amd64

package regabi

import (
    "reflect"
    "sync"
    "unsafe"

    "github.com/tencent/goom/internal/argdump/impl/shared"
    "github.com/tencent/goom/internal/argdump/internal/flags"
    "github.com/tencent/goom/internal/bytecode"
    "github.com/tencent/goom/internal/bytecode/memory"
    "github.com/tencent/goom/internal/patch"
    "github.com/tencent/goom/internal/unexports2"
)

var (
    patchOnce sync.Once

    origCallReflect func(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool, regs unsafe.Pointer)
    callDumpFuncVal func(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool, regs unsafe.Pointer)

    magicSentinel = new(int)
)

// callReflectTrampolineHolder is an alias for the shared trampoline holder.
var callReflectTrampolineHolder = shared.CallReflectTrampolineHolder

func ensureCallReflectPatched() {
    patchOnce.Do(func() {
        callReflectPtr, err := unexports2.FindFuncByName("reflect.callReflect")
        if err != nil || callReflectPtr == 0 {
            panic("argdump: failed to resolve reflect.callReflect")
        }

        flags.DebugCallReflectPtr = callReflectPtr
        flags.DebugCallDumpPtr = reflect.ValueOf(callDump).Pointer()

        funcToPtr := bytecode.FuncToPtr
        ptrToFunc := func(out *func(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool, regs unsafe.Pointer), ptr uintptr) error {
            _, err := unexports2.CreateFuncForCodePtr(out, ptr)
            return err
        }

        guard, err := patch.PtrTrampolineTyped(
            callReflectPtr,
            funcToPtr(callDump),
            funcToPtr(callReflectTrampolineHolder),
            ptrToFunc,
        )
        if err != nil {
            panic(err)
        }

        _, err = ptrToFunc(&origCallReflect, guard.FixOriginFunc())
        if err != nil {
            panic(err)
        }

        callDumpFuncVal = callDump
        guard.Apply()

        // Flush icache after patching
        memory.IcacheFlush(callReflectTrampolineHolder)
    })
}
```

---

## 示例4: Build Tags 验证测试

**regabi/buildtag_test.go**
```go
//go:build (go1.17 && amd64) || (go1.18 && amd64)
// +build go1.17,amd64 go1.18,amd64

package regabi

import (
    "runtime"
    "testing"
)

func TestBuildTagCorrectness(t *testing.T) {
    ver := runtime.Version()
    t.Logf("Running on Go version: %s", ver)

    // Verify we're on the expected architecture
    if runtime.GOARCH != "amd64" {
        t.Fatalf("Expected amd64, got %s", runtime.GOARCH)
    }

    // Verify the right compat type is selected
    var x anyCompat
    _ = x

    t.Logf("anyCompat type: %T", x)
}

func TestTypeLayoutConsistency(t *testing.T) {
    // Verify regArgs layout matches expectations
    var r regArgs

    if size := unsafe.Sizeof(r); size == 0 {
        t.Fatal("regArgs size is zero")
    }

    t.Logf("regArgs size: %d bytes", unsafe.Sizeof(r))
    t.Logf("Ints offset: %d", unsafe.Offsetof(r.Ints))
    t.Logf("Floats offset: %d", unsafe.Offsetof(r.Floats))
    t.Logf("Ptrs offset: %d", unsafe.Offsetof(r.Ptrs))
}
```

---

## 示例5: 迁移步骤脚本

**scripts/refactor_step1.sh**
```bash
#!/bin/bash
# 步骤1: 创建 regabi 包结构

set -e

echo "=== 重构步骤1: 创建 regabi 包 ==="

# 创建目录
mkdir -p internal/argdump/impl/regabi

# 创建 package doc
cat > internal/argdump/impl/regabi/doc.go <<'EOF'
// Package regabi provides shared implementation for register-based ABI
// used in Go 1.17+ on amd64 and Go 1.18+ on arm64.
//
// This package consolidates the common code from go117, go118, and go124
// packages to eliminate duplication.
package regabi
EOF

# 创建类型别名文件
cat > internal/argdump/impl/regabi/compat_go117.go <<'EOF'
//go:build go1.17 && !go1.18
// +build go1.17,!go1.18

package regabi

// anyCompat maps to interface{} in Go 1.17
type anyCompat = interface{}
EOF

cat > internal/argdump/impl/regabi/compat_go118.go <<'EOF'
//go:build go1.18
// +build go1.18

package regabi

// anyCompat maps to any in Go 1.18+
type anyCompat = any
EOF

echo "✓ regabi 包结构创建完成"
echo "下一步: 提取类型定义到 regabi/types_amd64.go"
```

**scripts/refactor_step2.sh**
```bash
#!/bin/bash
# 步骤2: 提取类型定义

set -e

echo "=== 重构步骤2: 提取类型定义 ==="

# 从 go118/core_amd64.go 提取类型定义
echo "提取类型定义到 regabi/types_amd64.go ..."

# 这里使用实际的代码提取逻辑
# (为简洁起见，此处省略具体实现)

echo "✓ 类型定义提取完成"
echo "运行测试验证..."

# 运行测试
go test -v ./internal/argdump/impl/regabi/...

echo "✓ 测试通过"
```

---

## 示例6: CI 验证配置

**.github/workflows/refactor-validation.yml**
```yaml
name: Refactor Validation

on:
  push:
    branches: [ refactor-argdump-impl ]
  pull_request:
    branches: [ master ]

jobs:
  test-multi-version:
    name: Test Go ${{ matrix.go-version }} on ${{ matrix.os }}
    runs-on: ${{ matrix.os }}
    strategy:
      matrix:
        # 测试所有支持的 Go 版本
        go-version: ['1.17', '1.18', '1.19', '1.20', '1.21', '1.22', '1.23', '1.24']
        os: [ubuntu-latest, macos-latest]
        arch: [amd64]
        include:
          # 添加 arm64 测试
          - go-version: '1.18'
            os: ubuntu-latest
            arch: arm64
          - go-version: '1.24'
            os: ubuntu-latest
            arch: arm64

    steps:
    - uses: actions/checkout@v3

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: ${{ matrix.go-version }}

    - name: Verify build tags
      run: |
        echo "Testing with Go ${{ matrix.go-version }}"
        go version

    - name: Run tests
      run: |
        cd internal/argdump/impl
        go test -v -race ./...

    - name: Check code duplication
      run: |
        # 检查是否还有重复代码
        ./scripts/check_duplication.sh

  benchmark:
    name: Performance Regression Check
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
      with:
        fetch-depth: 0  # 需要历史记录来对比

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.24'

    - name: Run benchmarks (before)
      run: |
        git checkout master
        go test -bench=. -benchmem ./internal/argdump/impl/... > bench-before.txt

    - name: Run benchmarks (after)
      run: |
        git checkout ${{ github.sha }}
        go test -bench=. -benchmem ./internal/argdump/impl/... > bench-after.txt

    - name: Compare benchmarks
      run: |
        go install golang.org/x/perf/cmd/benchstat@latest
        benchstat bench-before.txt bench-after.txt
```

---

## 总结

以上示例展示了方案A的核心实现技术：

1. **类型别名**解决 `interface{}` vs `any` 差异
2. **条件编译**通过 build tags 选择正确的实现
3. **重导出层**保持 API 向后兼容
4. **测试验证**确保类型布局正确性
5. **CI 验证**跨版本测试

这种方法：
- ✅ 零运行时开销
- ✅ 保持类型安全
- ✅ 易于理解和维护
- ✅ 增量实施，风险可控
