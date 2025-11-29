// pty_windows.go - Windows-specific PTY implementation
//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"github.com/ActiveState/termtest/conpty"
)

// Windows ConPTY wrapper
type WindowsPTY struct {
	cpty    *conpty.ConPty
	inPipe  *os.File
	outPipe *os.File
	process *os.Process
}

func (w *WindowsPTY) Write(data []byte) (int, error) {
	if w.inPipe != nil {
		return w.inPipe.Write(data)
	}
	return 0, fmt.Errorf("input pipe not available")
}

func (w *WindowsPTY) Read(data []byte) (int, error) {
	if w.outPipe != nil {
		return w.outPipe.Read(data)
	}
	return 0, fmt.Errorf("output pipe not available")
}

func (w *WindowsPTY) Close() error {
	var errs []error

	if w.inPipe != nil {
		if err := w.inPipe.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if w.outPipe != nil {
		if err := w.outPipe.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if w.cpty != nil {
		if err := w.cpty.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if w.process != nil {
		if err := w.process.Kill(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("multiple close errors: %v", errs)
	}
	return nil
}

func (w *WindowsPTY) Resize(cols, rows int) error {
	if w.cpty != nil {
		return w.cpty.Resize(uint16(cols), uint16(rows))
	}
	return fmt.Errorf("ConPTY not available")
}

// Windows ConPTY creation
func (t *NativeTerminalWidget) createWindowsPTY() (PTYInterface, error) {
	cpty, err := conpty.New(int16(t.cols), int16(t.rows))
	if err != nil {
		return nil, fmt.Errorf("failed to create ConPTY: %v", err)
	}

	systemRoot := os.Getenv("SYSTEMROOT")
	if systemRoot == "" {
		systemRoot = os.Getenv("WINDIR")
		if systemRoot == "" {
			systemRoot = "C:\\Windows"
		}
	}
	shell := filepath.Join(systemRoot, "System32", "cmd.exe")
	args := []string{}

	env := append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		fmt.Sprintf("COLUMNS=%d", t.cols),
		fmt.Sprintf("LINES=%d", t.rows),
		"ANSICON=1",
	)

	pid, _, err := cpty.Spawn(shell, args, &syscall.ProcAttr{
		Env: env,
	})
	if err != nil {
		cpty.Close()
		return nil, fmt.Errorf("failed to spawn shell: %v", err)
	}

	process, err := os.FindProcess(int(pid))
	if err != nil {
		cpty.Close()
		return nil, fmt.Errorf("failed to find process: %v", err)
	}

	log.Printf("Started %s with ConPTY (PID: %d)", shell, pid)

	return &WindowsPTY{
		cpty:    cpty,
		inPipe:  cpty.InPipe(),
		outPipe: cpty.OutPipe(),
		process: process,
	}, nil
}

// Stub for Unix function - not called on Windows
func (t *NativeTerminalWidget) createUnixPTY() (PTYInterface, error) {
	return nil, fmt.Errorf("Unix PTY not supported on Windows")
}
