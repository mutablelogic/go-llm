//go:build !client && !unix

package main

import "os"

// lockFile does nothing, as files aren't locked on this system, so only one
// process uses a store
func lockFile(*os.File) error {
	return nil
}

// unlockFile does nothing
func unlockFile(*os.File) error {
	return nil
}
