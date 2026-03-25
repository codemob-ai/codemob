package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

var ErrCancelled = errors.New("cancelled")

func ReadLine(fallback string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return readLineFallback(fallback)
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return readLineFallback(fallback)
	}
	defer term.Restore(fd, oldState)

	var buf []byte
	b := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(b)
		if err != nil {
			fmt.Print("\r\n")
			if len(buf) == 0 {
				return fallback, nil
			}
			return string(buf), nil
		}

		switch {
		case b[0] == 0x03:
			fmt.Print("\r\n")
			return "", ErrCancelled

		case b[0] == 0x1b:
			if isStandaloneEsc(fd) {
				fmt.Print("\r\n")
				return "", ErrCancelled
			}

		case b[0] == '\r' || b[0] == '\n':
			fmt.Print("\r\n")
			if len(buf) == 0 {
				return fallback, nil
			}
			return string(buf), nil

		case b[0] == 0x7f || b[0] == 0x08:
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				fmt.Print("\b \b")
			}

		case b[0] >= 0x80:
			consumeUTF8Continuation(b[0])

		case b[0] >= 0x20 && b[0] < 0x7f:
			buf = append(buf, b[0])
			fmt.Print(string(b[0]))
		}
	}
}

func Confirm(defaultYes bool) (bool, error) {
	input, err := ReadLine("")
	if err != nil {
		return false, err
	}
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return defaultYes, nil
	}
	return input == "y" || input == "yes", nil
}

// isStandaloneEsc reads ahead after an ESC byte to determine if it's a standalone
// ESC press (cancel) or the start of an escape sequence (arrow keys, etc.).
// Returns true if ESC is standalone, false if it's part of a sequence (consumed and discarded).
func isStandaloneEsc(fd int) bool {
	ch := make(chan byte, 1)
	go func() {
		b := make([]byte, 1)
		if _, err := os.Stdin.Read(b); err == nil {
			ch <- b[0]
		}
		close(ch)
	}()

	select {
	case next, ok := <-ch:
		if !ok {
			return true
		}
		if next == '[' || next == 'O' {
			drainSequence(fd, ch)
		}
		return false
	case <-time.After(50 * time.Millisecond):
		return true
	}
}

// drainSequence consumes the remaining bytes of a CSI or SS3 escape sequence.
func drainSequence(fd int, ch chan byte) {
	for {
		var next byte
		var ok bool

		select {
		case next, ok = <-ch:
			if !ok {
				return
			}
		default:
			b := make([]byte, 1)
			if _, err := os.Stdin.Read(b); err != nil {
				return
			}
			next = b[0]
		}

		// CSI sequences end with a byte in the 0x40-0x7E range
		if next >= 0x40 && next <= 0x7E {
			return
		}
	}
}

// consumeUTF8Continuation reads and discards the continuation bytes of a multi-byte
// UTF-8 character based on the leading byte.
func consumeUTF8Continuation(leading byte) {
	var remaining int
	switch {
	case leading&0xE0 == 0xC0:
		remaining = 1
	case leading&0xF0 == 0xE0:
		remaining = 2
	case leading&0xF8 == 0xF0:
		remaining = 3
	default:
		return
	}
	discard := make([]byte, remaining)
	os.Stdin.Read(discard)
}

func readLineFallback(fallback string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return fallback, nil
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return fallback, nil
	}
	return input, nil
}
