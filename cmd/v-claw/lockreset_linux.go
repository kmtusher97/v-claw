package main

import "fmt"

// lockReset has nothing to reset: the virtual lock has no Linux implementation yet.
func lockReset() error {
	return fmt.Errorf("the virtual lock is not implemented on Linux yet")
}
