# `internal/abijson` (POC)

This directory contains a **Go 1.24 + arm64** proof-of-concept JSON encoder that:

- Takes `(reflect.Type, unsafe.Pointer)` and **encodes JSON by reading memory** (no `reflect.Value`).
- Includes a **forked (noswiss) map iterator** based on Go 1.24 runtime layout.

## Build constraints

- `//go:build go1.24 && arm64`
- Assumes `!goexperiment.swissmap` (i.e. Go 1.24 noswiss map layout).

## API

- `EncodeJSONFromAddr(t reflect.Type, addr unsafe.Pointer) ([]byte, error)`

## Notes

- This is intentionally a POC. It does not attempt to be safe against concurrent map writes.
- Pointer dereferences are best-effort; invalid pointers will be rendered as a string marker.


