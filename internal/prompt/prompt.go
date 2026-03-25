package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

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
		case b[0] == 0x03 || b[0] == 0x1b:
			fmt.Print("\r\n")
			return "", ErrCancelled

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
