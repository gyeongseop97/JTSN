package main

import "unsafe"

// winPtr preserves a Win32 pointer-sized value as a pointer. The implementation
// lives in assembly so go vet does not mistake Windows message pointers for
// pointer arithmetic.
func winPtr(value uintptr) unsafe.Pointer
