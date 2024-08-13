//go:build !unix
// +build !unix

package main

import (
	"fmt"
	"syscall"
)

func reusePort(s uintptr) {
	if errReuse := syscall.SetsockoptInt(syscall.Handle(s), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); errReuse != nil {
		fmt.Printf("reuse addr error: %v\n", errReuse)
		return
	}

	if errReusePort := syscall.SetsockoptInt(syscall.Handle(s), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); errReusePort != nil {
		fmt.Printf("reuse port error: %v\n", errReusePort)
		return
	}
}
