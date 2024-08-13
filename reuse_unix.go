//go:build unix
// +build unix

package main

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

func reusePort(s uintptr) {
	if errReuse := syscall.SetsockoptInt(int(s), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); errReuse != nil {
		fmt.Printf("reuse addr error: %v\n", errReuse)
		return
	}

	if errReusePort := syscall.SetsockoptInt(int(s), syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1); errReusePort != nil {
		fmt.Printf("reuse port error: %v\n", errReusePort)
		return
	}
}
