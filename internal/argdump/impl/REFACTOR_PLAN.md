# argdump/impl 重构方案设计

## 一、现状分析

### 1.1 代码重复统计

| 版本 | 架构 | 核心代码行数 | 重复度 |
|------|------|-------------|--------|
| go117 | amd64 | 690 | 100% (与go118相同，仅`interface{}`差异) |
| go118 | amd64 | 690 | 100% (与go124完全相同) |
| go124 | amd64 | 690 | 100% (与go118完全相同) |
| pre117 | 通用 | 275 | 独立实现 (Stack-only ABI) |

### 1.2 关键重复代码区域

**完全重复的代码块（go117/go118/go124 amd64）：**

1. **类型定义** (~70行)
   - `regArgs` 结构体
   - `makeFuncCtxt` 结构体
   - `DumpFuncImpl` 结构体
   - `abiStep`, `abiSeq`, `abiDesc` 类型

2. **核心函数** (~450行)
   - `ensureCallReflectPatched()`: 56行，完全相同
   - `callDump()`: ~30行，完全相同
   - `dumpArgsAndZeroRets()`: 105行，仅 `[]interface{}` vs `[]any` 差异
   - `dumpReturns()`: ~60行，仅 `[]interface{}` vs `[]any` 差异
   - ABI 处理逻辑: ~250行，完全相同

3. **辅助函数** (~170行)
   - `addrOfArg()`, `addrOfRet()`: 仅返回值类型差异
   - `newAbiDescFromFuncType()`: 完全相同

### 1.3 架构差异点

| 特性 | pre117 | go117 | go118+ |
|------|--------|-------|--------|
| ABI 模型 | Stack-only | amd64:Register; arm64:Stack | Register (both) |
| `callReflect` 签名 | 3参数 | 4参数 (regs) | 4参数 (regs) |
| `any` 关键字 | ❌ | ❌ | ✅ |
| 寄存器处理 | ❌ | amd64:✅; arm64:❌ | ✅ |

---

## 二、重构方案设计

### 方案A：Build Tags 增量优化（推荐）

#### 核心思路
- **合并 go118 和 go124**：因为完全相同
- **提取 amd64 公共代码**：通过条件编译处理 `interface{}` vs `any` 差异
- **保持 pre117 独立**：因为 ABI 模型根本不同
- **arm64 部分提取**：共享类型定义，独立实现 ABI 逻辑

#### 目录结构
```
internal/argdump/impl/
├── shared/                      # 跨版本共享
│   ├── bitmap.go               # IntArgRegBitmap
│   ├── panic.go                # DumpPanic
│   ├── trampoline.go           # Go1.17+ 蹦床
│   └── trampoline_pre117.go    # Go1.13-1.16 蹦床
│
├── regabi/                      # 寄存器 ABI 共享实现 (go1.17+)
│   ├── types_amd64.go          # amd64 类型定义 (go1.17+)
│   ├── types_arm64.go          # arm64 类型定义 (go1.18+)
│   ├── core_amd64.go           # amd64 核心实现
│   ├── core_amd64_go117.go     # go1.17 特定差异
│   ├── core_amd64_go118.go     # go1.18+ 特定差异 (any关键字)
│   ├── core_arm64.go           # arm64 核心实现 (go1.18+)
│   ├── patch_amd64.go          # amd64 patch 函数
│   ├── patch_arm64.go          # arm64 patch 函数
│   └── entry_arm64.s           # arm64 汇编入口
│
├── go117/                       # Go 1.17 特定 (主要是arm64 stack-only)
│   ├── core_arm64.go           # arm64 stack-only 实现
│   ├── patch_arm64.go          # arm64 patch
│   ├── entry_arm64.s           # arm64 汇编
│   ├── flags.go                # 导出标志
│   ├── stub_other.go           # 不支持的架构
│   └── doc.go
│
├── go118/                       # Go 1.18+ (合并原 go118/go124)
│   ├── flags.go                # 导出标志
│   ├── stub_other.go           # 不支持的架构
│   └── doc.go
│
├── pre117/                      # Go 1.13-1.16 (保持独立)
│   └── ... (保持现有结构)
│
└── ... (build tag 路由文件)
```

#### Build Tags 策略

**regabi/core_amd64.go**
```go
//go:build (go1.17 && !go1.18 && amd64) || (go1.18 && amd64)
// +build go1.17,!go1.18,amd64 go1.18,amd64

package regabi
```

**regabi/core_amd64_go117.go** (处理 `interface{}` 特定代码)
```go
//go:build go1.17 && !go1.18 && amd64
// +build go1.17,!go1.18,amd64

package regabi

// 仅包含使用 interface{} 的函数
func dumpArgsAndZeroRets_impl(...) {
    var keepAlive []interface{}  // go1.17 使用 interface{}
    // ...
}
```

**regabi/core_amd64_go118.go** (处理 `any` 特定代码)
```go
//go:build go1.18 && amd64
// +build go1.18,amd64

package regabi

// 仅包含使用 any 的函数
func dumpArgsAndZeroRets_impl(...) {
    var keepAlive []any  // go1.18+ 使用 any
    // ...
}
```

#### 实施步骤

1. **阶段1：创建 regabi 包** (低风险)
   - 创建 `regabi/` 目录
   - 提取 amd64 公共类型定义到 `types_amd64.go`
   - 添加测试验证类型布局正确性

2. **阶段2：迁移 amd64 核心代码** (中风险)
   - 将 `core_amd64.go` 主体迁移到 `regabi/core_amd64.go`
   - 创建 `core_amd64_go117.go` 和 `core_amd64_go118.go` 处理差异
   - 更新 `go117/`, `go118/` 包为重导出层
   - 运行完整测试套件

3. **阶段3：合并 go118 和 go124** (低风险)
   - 删除 `go124/` 目录
   - 更新 `go118/` 的 build tags 为 `go1.18` (移除 `!go1.24`)
   - 验证 go1.24 环境下测试通过

4. **阶段4：优化 arm64** (可选)
   - 提取 go1.18+ arm64 的公共代码到 regabi
   - 保持 go1.17 arm64 独立（stack-only ABI）

5. **阶段5：清理和文档** (低风险)
   - 移除冗余文件
   - 更新包文档
   - 添加架构决策记录 (ADR)

#### 预期收益

- **代码减少**: ~1380 行 (690×2，合并 go118/go124 amd64)
- **维护成本**: 减少 60%+ (单一实现点)
- **bug 修复**: 一次修复，所有版本受益
- **可读性**: 架构分层更清晰

---

### 方案B：特性标志（Feature Flags）+ 接口抽象

#### 核心思路
- 定义统一的 ABI 接口
- 运行时选择实现（而非编译时）
- 使用特性标志控制差异行为

#### 接口设计
```go
// ABI 接口定义
type ABIStrategy interface {
    // 从寄存器/栈中读取参数地址
    AddrOfArg(i int, t reflect.Type, frame unsafe.Pointer, regs unsafe.Pointer) (unsafe.Pointer, any)

    // 从寄存器/栈中读取返回值地址
    AddrOfRet(i int, t reflect.Type, frame unsafe.Pointer, regs unsafe.Pointer) (unsafe.Pointer, any)

    // 清零返回值
    ZeroReturns(frame unsafe.Pointer, regs unsafe.Pointer)

    // 计算栈帧大小
    StackFrameSize() uintptr
}

// 特性标志
type Features struct {
    HasRegisterABI  bool  // 是否支持寄存器 ABI
    HasAnyKeyword   bool  // 是否有 any 关键字
    IntArgRegs      int   // 整数寄存器数量
    FloatArgRegs    int   // 浮点寄存器数量
}

// 版本特性
var versionFeatures = Features{
    HasRegisterABI: true,  // go1.18+
    HasAnyKeyword:  true,  // go1.18+
    IntArgRegs:     9,     // amd64: 9, arm64: 8
    FloatArgRegs:   15,    // amd64: 15, arm64: 16
}
```

#### 优点
- 最大化代码复用
- 易于添加新架构/版本
- 运行时灵活性高

#### 缺点
- 性能开销（虚函数调用）
- 复杂度高
- 改动范围大

#### 适用场景
如果需要在**运行时**动态选择 ABI 策略（例如支持多版本 Go 的单一二进制），选择此方案。

---

### 方案C：代码生成（Code Generation）

#### 核心思路
- 编写代码生成器
- 使用模板 + 参数生成各版本实现
- 保持生成代码可读性

#### 模板示例
```go
// gen/template/core_amd64.go.tmpl
{{define "core"}}
//go:build {{.BuildTag}}

package {{.Package}}

{{if .UseAny}}
func dumpArgsAndZeroRets(...) {
    var keepAlive []any
{{else}}
func dumpArgsAndZeroRets(...) {
    var keepAlive []interface{}
{{end}}
    // ... 共同逻辑
}
{{end}}
```

#### 配置文件
```yaml
# gen/versions.yaml
versions:
  - name: go117
    package: go117
    buildTag: "go1.17 && !go1.18 && amd64"
    features:
      useAny: false
      hasRegisterABI: true

  - name: go118
    package: go118
    buildTag: "go1.18 && amd64"
    features:
      useAny: true
      hasRegisterABI: true
```

#### 优点
- DRY (Don't Repeat Yourself)
- 生成代码可审查
- 易于批量修改

#### 缺点
- 需要维护生成器
- 调试复杂（需要看生成后的代码）
- 学习曲线

---

## 三、方案对比与推荐

| 维度 | 方案A (Build Tags) | 方案B (接口) | 方案C (代码生成) |
|------|-------------------|-------------|-----------------|
| 实施难度 | ⭐⭐ 中等 | ⭐⭐⭐⭐ 高 | ⭐⭐⭐ 中高 |
| 代码减少 | ~60% | ~70% | ~80% |
| 性能影响 | 无 | 小（虚函数） | 无 |
| 可维护性 | ⭐⭐⭐⭐ 好 | ⭐⭐⭐ 中等 | ⭐⭐⭐ 中等 |
| 风险 | ⭐⭐ 低 | ⭐⭐⭐⭐ 高 | ⭐⭐⭐ 中 |
| 向后兼容 | ✅ 完全兼容 | ✅ 完全兼容 | ✅ 完全兼容 |

### 推荐：**方案A (Build Tags 增量优化)**

#### 理由
1. **平衡收益与风险**: 可消除 60%+ 重复代码，风险可控
2. **增量实施**: 可分阶段进行，每阶段都可回滚
3. **零性能损失**: 编译时选择，无运行时开销
4. **符合 Go 惯例**: Build tags 是 Go 标准做法
5. **易于理解**: 团队成员容易理解和维护

---

## 四、实施路线图

### 里程碑1：准备阶段 (1-2天)
- [ ] 评审本设计文档
- [ ] 建立完整的测试覆盖（确保重构安全）
- [ ] 创建重构分支

### 里程碑2：regabi 包基础 (2-3天)
- [ ] 创建 `regabi/` 目录结构
- [ ] 提取类型定义 (`types_amd64.go`)
- [ ] 验证类型布局测试通过

### 里程碑3：amd64 核心迁移 (3-5天)
- [ ] 迁移 `core_amd64.go` 主体
- [ ] 创建版本特定文件 (go117/go118)
- [ ] 更新 go117/go118 包为重导出
- [ ] 完整回归测试

### 里程碑4：合并 go118/go124 (1天)
- [ ] 删除 `go124/` 目录
- [ ] 更新 build tags
- [ ] 测试 go1.24 环境

### 里程碑5：优化 patch 和 arm64 (可选，2-3天)
- [ ] 提取 patch 函数公共代码
- [ ] 优化 arm64 实现（如需要）

### 里程碑6：清理和文档 (1-2天)
- [ ] 删除冗余文件
- [ ] 更新包文档和注释
- [ ] 编写架构决策记录 (ADR)
- [ ] Code review 和合并

**总预估**: 10-16 工作日

---

## 五、风险与缓解

### 风险1: Build Tags 组合错误
**影响**: 编译失败或错误版本被选择
**缓解**:
- 添加编译时检查（使用 `//go:build` 和 `// +build` 双重验证）
- CI 测试所有 Go 版本（1.13, 1.17, 1.18, 1.20, 1.24）

### 风险2: 类型布局不匹配
**影响**: 运行时 panic 或数据损坏
**缓解**:
- 添加 `unsafe.Sizeof()` 和 `unsafe.Offsetof()` 测试
- 使用 `//go:linkname` 验证与 runtime 结构体对齐

### 风险3: 回归 bug
**影响**: 已修复的 bug 重新出现
**缓解**:
- 完整的单元测试和集成测试
- 保留原有所有测试用例
- 分阶段迁移，每阶段测试

### 风险4: 性能退化
**影响**: 参数捕获性能下降
**缓解**:
- 添加性能基准测试
- 对比重构前后性能数据
- 确保零抽象成本（编译时选择）

---

## 六、后续优化方向

1. **统一测试框架**: 为所有版本共享测试用例
2. **文档生成**: 自动生成版本兼容性矩阵
3. **CI 增强**: 添加跨版本兼容性测试
4. **性能优化**: 优化寄存器参数读取路径
5. **支持新架构**: 为 riscv64, loong64 做准备

---

## 七、附录

### A. 文件映射关系

#### 重构前
```
go117/core_amd64.go (690行)
go118/core_amd64.go (690行) ← 与 go117 99% 相同
go124/core_amd64.go (690行) ← 与 go118 100% 相同
```

#### 重构后
```
regabi/core_amd64.go (650行)          ← 公共实现
regabi/core_amd64_go117.go (30行)     ← go1.17 差异
regabi/core_amd64_go118.go (30行)     ← go1.18+ 差异
```

**代码减少**: 690×3 - (650+30+30) = 1360 行 (66% 减少)

### B. 关键差异代码示例

**interface{} vs any 差异**
```go
// go1.17
func (a abiDesc) addrOfArg(...) (unsafe.Pointer, interface{}) { ... }

// go1.18+
func (a abiDesc) addrOfArg(...) (unsafe.Pointer, any) { ... }
```

**解决方案: 类型别名**
```go
// regabi/compat_go117.go
//go:build go1.17 && !go1.18

type anyCompat = interface{}

// regabi/compat_go118.go
//go:build go1.18

type anyCompat = any

// regabi/core_amd64.go
func (a abiDesc) addrOfArg(...) (unsafe.Pointer, anyCompat) { ... }
```

---

**文档版本**: v1.0
**创建日期**: 2026-02-08
**作者**: Claude Code
**状态**: 待评审
