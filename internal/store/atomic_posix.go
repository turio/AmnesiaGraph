//go:build !windows

package store

import "os"

func atomicRename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
