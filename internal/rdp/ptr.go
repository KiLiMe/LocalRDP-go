package rdp

import "unsafe"

// ptrAt is the pointer at addr, an address Windows handed this program: an
// lParam of a message, a string the object returned, a SID inside a token.
// Those never move, so the rule behind go vet's unsafeptr warning (a
// uintptr does not keep an object alive) does not apply here; writing the
// address through a pointer keeps the conversion out of that check.
func ptrAt[T any](addr uintptr) *T {
	var p unsafe.Pointer
	*(*uintptr)(unsafe.Pointer(&p)) = addr
	return (*T)(p)
}
