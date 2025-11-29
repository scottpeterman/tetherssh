// pty_unix.go - Unix-specific PTY implementation
//go:build !windows

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"

	"github.com/creack/pty" // Unix PTY
)

// Unix PTY creation
func (t *NativeTerminalWidget) createUnixPTY() (PTYInterface, error) {
	var shell string
	var args []string

	// Determine shell
	shell = os.Getenv("SHELL")
	if shell == "" {
		if runtime.GOOS == "darwin" {
			shell = "/bin/zsh"
		} else {
			shell = "/bin/bash"
		}
	}

	// Create command
	cmd := exec.Command(shell, args...)

	// Enhanced environment setup
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"FORCE_COLOR=1",
		"CLICOLOR=1",
		"CLICOLOR_FORCE=1",
		fmt.Sprintf("COLUMNS=%d", t.cols),
		fmt.Sprintf("LINES=%d", t.rows),
		"LC_ALL=C.UTF-8",
		"LANG=C.UTF-8",
	)

	if runtime.GOOS == "darwin" {
		cmd.Env = append(cmd.Env, "TERM_PROGRAM=Terminal")
	}

	// Start PTY with command
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start shell: %v", err)
	}

	// Set initial PTY size
	pty.Setsize(ptmx, &pty.Winsize{
		Rows: uint16(t.rows),
		Cols: uint16(t.cols),
	})

	log.Printf("Started %s with PTY", shell)

	return &UnixPTY{
		ptyFile: ptmx,
		cmd:     cmd,
	}, nil
}

// Stub for Windows function - this will not be compiled on Unix systems
// but provides a fallback if somehow called
func (t *NativeTerminalWidget) createWindowsPTY() (PTYInterface, error) {
	return nil, fmt.Errorf("Windows PTY not supported on this platform")
}
