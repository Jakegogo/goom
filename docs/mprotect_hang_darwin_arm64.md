## darwin/arm64 mprotect hang during Apply

### Summary
On darwin/arm64, `Apply` can hang when patching a function whose code resides on the
same text page as the currently executing test code. The hang occurs inside
`mprotect(PROT_READ|PROT_WRITE)` and blocks the process indefinitely.

### Symptoms
- Test output stops at `[test] Apply start`.
- CPU usage may drop to idle (blocked thread), no panic or crash.
- `GOOM_TRACE_MPROTECT=1` shows `RW start` with no matching `RW ok`.

Example trace:
```
[test] Apply start
[mprotect] RW start addr=0x1043b8000 len=0x4000
```

### Investigation (timeline)
1. `sample` showed threads parked in kernel wait (no busy loop).
2. Added `GOOM_TRACE_MPROTECT` to print before/after `mprotect`.
3. Identified the hang at the first `mprotect(RW)` in `Apply`.
4. Reproduced only when patching local functions compiled into the same text page.
5. Cross-package targets (`internal/argdump/testtargets`) did not hang.

### Root cause
On macOS arm64, changing permissions on a text page that contains currently executing
code can block indefinitely. When the target function is in the same page as the test
code that is calling `Apply`, `mprotect` may stall while the kernel waits for a safe
state. This is a platform/ABI-specific behavior tied to W^X and code page protection.

### Fix and mitigations
1. **Pre-check before mprotect**
   - `internal/bytecode/memory/writeTo` now scans the current call stack and aborts
     if any caller PC lies on the same text page being patched.
   - This avoids hard hangs and returns a descriptive error instead.
2. **Use cross-package patch targets in tests**
   - Tests default to patching `internal/argdump/testtargets` functions to reduce
     the chance of patching the currently executing text page.

### Controls and diagnostics
- `GOOM_TRACE_MPROTECT=1`  
  Prints `mprotect` begin/end for each page to stderr.
- `GOOM_ALLOW_TEXT_MPROTECT=1`  
  Disables the safety pre-check (for controlled reproduction).

### Why this is safe
The pre-check only blocks the case known to hang on darwin/arm64. Other platforms
and cross-package patch targets continue to work normally.

### References
- `internal/bytecode/memory/mwrite_prot.go`
- `internal/argdump/patchfunc_go118_to_go123_arm64_test.go`
