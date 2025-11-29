//go:build windows
// +build windows

// gopyte/cli/cmdpty_demo_windows.go
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"os/user"
	"time"

	gopyte "github.com/scottpeterman/gopyte/gopyte"

	"github.com/UserExistsError/conpty"
	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

func main() {
	enableVT()

	cols, rows := 80, 24
	c0, r0, err := term.GetSize(int(os.Stdout.Fd()))
	if err == nil && c0 > 0 && r0 > 0 {
		cols, rows = c0, r0
	}

	// spawn cmd.exe inside ConPTY (with initial dimensions)
	cpty, err := conpty.Start(`C:\Windows\System32\cmd.exe`, conpty.ConPtyDimensions(cols, rows))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ConPTY start failed: %v\n", err)
		if conpty.IsConPtyAvailable() == false {
			fmt.Fprintln(os.Stderr, "This Windows build doesn’t support ConPTY (needs 1809+).")
		}
		os.Exit(1)
	}
	defer cpty.Close()

	// make our console raw so keystrokes flow straight to the child
	oldState, rErr := term.MakeRaw(int(os.Stdin.Fd()))
	if rErr == nil {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}

	// build gopyte emulator
	screen := gopyte.NewWideCharScreen(cols, rows, 2000)
	stream := gopyte.NewStream(screen, false)

	// PTY -> gopyte
	go func() {
		br := bufio.NewReader(cpty)
		buf := make([]byte, 4096)
		for {
			n, err := br.Read(buf)
			if n > 0 {
				stream.Feed(string(buf[:n]))
				redraw(screen)
			}
			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(os.Stderr, "pty read error: %v\n", err)
				}
				return
			}
		}
	}()

	// stdin -> PTY
	go func() {
		in := bufio.NewReader(os.Stdin)
		b := make([]byte, 1)
		for {
			_, err := in.Read(b)
			if err != nil {
				return
			}
			_, _ = cpty.Write(b)
		}
	}()

	// periodic resize polling (Windows doesn’t emit SIGWINCH)
	go func() {
		lastC, lastR := cols, rows
		t := time.NewTicker(250 * time.Millisecond)
		defer t.Stop()
		for range t.C {
			c, r, err := term.GetSize(int(os.Stdout.Fd()))
			if err == nil && c > 0 && r > 0 {
				if c != lastC || r != lastR {
					lastC, lastR = c, r
					_ = cpty.Resize(c, r)
					screen.Resize(c, r)
					redraw(screen)
				}
			}
		}
	}()

	// ctrl+c etc: just repaint or exit when the child exits
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		for {
			_, ok := <-sigCh
			if !ok {
				return
			}
			redraw(screen)
		}
	}()

	// greet
	username := "(you)"
	if u, _ := user.Current(); u != nil && u.Username != "" {
		username = u.Username
	}
	_, _ = cpty.Write([]byte("echo. & echo :: gopyte + ConPTY ::\r\n"))
	_, _ = cpty.Write([]byte("echo Type commands here. Resize the window to test screen.Resize().\r\n"))
	_, _ = cpty.Write([]byte("echo User: " + username + "\r\n"))

	// wait for child to exit
	_, _ = cpty.Wait(context.Background())
}

func redraw(screen *gopyte.WideCharScreen) {
	_, _ = os.Stdout.WriteString("\x1b[2J\x1b[H") // clear + home
	lines := screen.GetDisplay()
	i := 0
	for i < len(lines) {
		_, _ = os.Stdout.WriteString(lines[i])
		_, _ = os.Stdout.WriteString("\r\n")
		i++
	}
}

func enableVT() {
	stdout := windows.Handle(os.Stdout.Fd())
	var mode uint32
	_ = windows.GetConsoleMode(stdout, &mode)
	const (
		ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004
		ENABLE_PROCESSED_OUTPUT            = 0x0001
	)
	mode |= ENABLE_VIRTUAL_TERMINAL_PROCESSING | ENABLE_PROCESSED_OUTPUT
	_ = windows.SetConsoleMode(stdout, mode)
}
