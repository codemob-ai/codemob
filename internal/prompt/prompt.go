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

var fallbackReader *bufio.Reader

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
			if isStandaloneEsc() {
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
			fmt.Print("\a")

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
//
// Note: when the timeout fires (standalone ESC), the reader goroutine remains blocked on
// os.Stdin.Read and will swallow the next byte that arrives. This is safe because the
// cancel path exits cleanly without further stdin reads.
func isStandaloneEsc() bool {
	ch := make(chan byte, 1)
	go func() {
		b := make([]byte, 1)
		if _, err := os.Stdin.Read(b); err == nil {
			ch <- b[0]
		}
	}()

	select {
	case next := <-ch:
		if next == '[' || next == 'O' {
			drainSequence()
		}
		return false
	case <-time.After(50 * time.Millisecond):
		return true
	}
}

// drainSequence consumes the remaining bytes of a CSI or SS3 escape sequence
// directly from stdin.
func drainSequence() {
	b := make([]byte, 1)
	for {
		if _, err := os.Stdin.Read(b); err != nil {
			return
		}
		if b[0] >= 0x40 && b[0] <= 0x7E {
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
	if fallbackReader == nil {
		fallbackReader = bufio.NewReader(os.Stdin)
	}
	input, err := fallbackReader.ReadString('\n')
	if err != nil {
		return fallback, nil
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return fallback, nil
	}
	return input, nil
}
