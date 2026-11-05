//go:build !client && unix

package main

import (
	"os"
	"syscall"
)

// lockFile locks the file, which is open, so no other process, which locks
// it, has it, until it's unlocked, waiting for one which has it
func lockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// unlockFile unlocks the file
func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
