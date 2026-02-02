# Compatibility Plan (Go versions & CPU architectures)

This repository has **two different layers** of “patching” that must not be conflated:

- **Core patch engine**: `internal/patch` (monkey patch / jump stubs / optional trampoline).
- **Advanced reflect-based instrumentation**: `internal/argdump` (patches `reflect.callReflect`, relies on `reflect.makeFuncStub`, ABI details, stack maps).

The **core patch engine** is the priority for broad Go-version and CPU-arch support.
`argdump` is intentionally more conservative because it depends on very sensitive runtime/ABI behavior.

---

## Go version plan

### Target in this iteration (current request)

- **Support floor**: **Go 1.17+** for the “core patch engine” (`internal/patch`) across Tier-1 architectures.
- `internal/argdump` advanced PatchFunc/hook path remains **go1.24+** for now (see below).

### Next iteration (agreed future)

- Extend the **core patch engine** support floor down to **Go 1.13+**.
  - Focus on ABI0-era differences, symbol table differences, and trampoline relocation coverage.

---

## CPU architecture plan

### Tier-1 (must pass CI)

- `linux/amd64`, `darwin/amd64`
- `linux/arm64`, `darwin/arm64`

### Tier-2 (best effort)

- `linux/386` (and other 32-bit where feasible)

### Tier-3 (future)

- `ppc64le`, `riscv64`, `s390x`, etc.

---

## Support matrix (high-level)

Legend:
- **Core** = `internal/patch` Patch/Ptr/InstanceMethod jump + (optional) trampoline
- **Argdump** = `internal/argdump` reflect-hook-based dumping/continuation

| Go version | Core patch engine | Arggdump MakeDumpFunc | Arggdump PatchFunc |
|---|---|---|---|
| go1.13–go1.16 | planned (next iteration) | supported (already) | not planned short-term |
| go1.17 | **target now** | supported | not yet (kept conservative) |
| go1.18–go1.23 | **target now** | supported | not yet (kept conservative) |
| go1.24+ | supported | supported | supported |

---

## Arm64 stability rules (core patch engine)

These are **non-negotiable** rules for arm64 entry stubs and are enforced by tests:

- **Do not clobber argument registers**: `x0-x15` may hold parameters (regabi).
- **Correct funcval context register**: when branching to a func value, `x26` must point to the funcval/closure context.
- **Scratch regs**: use `x16/x17` (IP0/IP1) for scratch work in stubs.

Violating these tends to surface as:
- wrong arguments/returns
- SIGBUS/SIGSEGV
- crashes inside `reflect.callReflect` / `reflect.funcLayout` due to corrupted ctxt

Relevant code: `internal/patch/monkey_arm64.go`, `internal/patch/fix_addr_arm64.go`.

---

## Testing strategy

Default tests (`go test ./...`) must be deterministic and **not depend on**:
- downloading toolchains
- network services / fixed local ports
- fragile symbol-table availability in test binaries

Therefore some tests are opt-in via env vars:
- Multi-toolchain compatibility tests: `GOOM_ENABLE_COMPAT_TEST=1`
- Unexported var symbol-table tests: `GOOM_ENABLE_UNEXPORTED_VAR_TEST=1`

