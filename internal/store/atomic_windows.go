//go:build windows

package store

import (
	"syscall"
	"unsafe"
)

const (
	moveFileReplaceExisting = 0x1
	moveFileWriteThrough    = 0x8
)

var moveFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

func atomicRename(oldPath, newPath string) error {
	oldUTF16, err := syscall.UTF16PtrFromString(oldPath)
	if err != nil {
		return err
	}
	newUTF16, err := syscall.UTF16PtrFromString(newPath)
	if err != nil {
		return err
	}
	result, _, callErr := moveFileEx.Call(
		uintptr(unsafe.Pointer(oldUTF16)),
		uintptr(unsafe.Pointer(newUTF16)),
		moveFileReplaceExisting|moveFileWriteThrough,
	)
	if result == 0 {
		return callErr
	}
	return nil
}
