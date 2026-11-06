// Package terminal reads a secret from the controlling terminal without
// echo, restoring the terminal even when interrupted.
package terminal

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"unsafe"
)

// ErrNoTerminal reports a process without a controlling terminal
// (a systemd service, a pipe, a cron job).
var ErrNoTerminal = errors.New("no terminal to ask on")

// maxSecretBytes caps one secret typed by a person.
const maxSecretBytes = 4096

// IsTerminal reports whether file is a terminal.
func IsTerminal(file *os.File) bool {
	var state syscall.Termios
	return ioctl(file.Fd(), syscall.TCGETS, &state) == nil
}

// ReadSecret prints prompt on /dev/tty and reads one line without echo.
func ReadSecret(prompt string) ([]byte, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, ErrNoTerminal
	}
	defer tty.Close()
	var saved syscall.Termios
	if err := ioctl(tty.Fd(), syscall.TCGETS, &saved); err != nil {
		return nil, ErrNoTerminal
	}
	quiet := saved
	quiet.Lflag &^= syscall.ECHO
	quiet.Lflag |= syscall.ICANON | syscall.ISIG
	if err := ioctl(tty.Fd(), syscall.TCSETS, &quiet); err != nil {
		return nil, fmt.Errorf("turn echo off: %w", err)
	}
	restore := func() { _ = ioctl(tty.Fd(), syscall.TCSETS, &saved) }
	defer restore()

	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(interrupted)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-interrupted:
			restore()
			fmt.Fprintln(tty)
			os.Exit(130)
		case <-done:
		}
	}()

	fmt.Fprint(tty, prompt)
	line, err := bufio.NewReaderSize(tty, maxSecretBytes).ReadString('\n')
	fmt.Fprintln(tty)
	if err != nil {
		return nil, fmt.Errorf("read from the terminal: %w", err)
	}
	return []byte(strings.TrimRight(line, "\r\n")), nil
}

func ioctl(fd uintptr, request uintptr, state *syscall.Termios) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, uintptr(unsafe.Pointer(state)))
	if errno != 0 {
		return errno
	}
	return nil
}
