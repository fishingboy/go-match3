//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "The GUI version uses raw Win32/GDI and only runs on Windows.")
	fmt.Fprintln(os.Stderr, "Try the terminal version instead: go run ./cmd/cli")
	os.Exit(1)
}
