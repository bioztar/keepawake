//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "keepawake.exe is Windows-only; on macOS use macos/keepawake")
	os.Exit(1)
}
