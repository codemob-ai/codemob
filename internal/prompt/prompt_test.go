package prompt

import (
	"os"
	"strings"
	"testing"
)

func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdin
	origReader := fallbackReader
	os.Stdin = r
	fallbackReader = nil
	t.Cleanup(func() {
		os.Stdin = orig
		fallbackReader = origReader
	})

	w.WriteString(input)
	w.Close()
	fn()
}

func TestReadLineFallback_EmptyInput(t *testing.T) {
	withStdin(t, "\n", func() {
		got, err := readLineFallback("default-val")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "default-val" {
			t.Errorf("got %q, want %q", got, "default-val")
		}
	})
}

func TestReadLineFallback_WithInput(t *testing.T) {
	withStdin(t, "custom\n", func() {
		got, err := readLineFallback("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "custom" {
			t.Errorf("got %q, want %q", got, "custom")
		}
	})
}

func TestReadLineFallback_EOF(t *testing.T) {
	withStdin(t, "", func() {
		got, err := readLineFallback("fallback")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "fallback" {
			t.Errorf("got %q, want %q", got, "fallback")
		}
	})
}

func TestReadLineFallback_WhitespaceOnly(t *testing.T) {
	withStdin(t, "   \n", func() {
		got, err := readLineFallback("fallback")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "fallback" {
			t.Errorf("got %q, want %q", got, "fallback")
		}
	})
}

func TestReadLine_PipedInput(t *testing.T) {
	withStdin(t, "hello\n", func() {
		got, err := ReadLine("fallback")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "hello" {
			t.Errorf("got %q, want %q", got, "hello")
		}
	})
}

func TestReadLine_PipedEmptyInput(t *testing.T) {
	withStdin(t, "\n", func() {
		got, err := ReadLine("fallback")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "fallback" {
			t.Errorf("got %q, want %q", got, "fallback")
		}
	})
}

func TestReadLine_PipedMultipleCalls(t *testing.T) {
	withStdin(t, "first\nsecond\nthird\n", func() {
		got1, err := ReadLine("default1")
		if err != nil {
			t.Fatalf("call 1: unexpected error: %v", err)
		}
		if got1 != "first" {
			t.Errorf("call 1: got %q, want %q", got1, "first")
		}

		got2, err := ReadLine("default2")
		if err != nil {
			t.Fatalf("call 2: unexpected error: %v", err)
		}
		if got2 != "second" {
			t.Errorf("call 2: got %q, want %q", got2, "second")
		}

		got3, err := ReadLine("default3")
		if err != nil {
			t.Fatalf("call 3: unexpected error: %v", err)
		}
		if got3 != "third" {
			t.Errorf("call 3: got %q, want %q", got3, "third")
		}
	})
}

func TestConfirm_Yes(t *testing.T) {
	for _, input := range []string{"y\n", "Y\n", "yes\n", "YES\n"} {
		t.Run(strings.TrimSpace(input), func(t *testing.T) {
			withStdin(t, input, func() {
				got, err := Confirm(false)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !got {
					t.Errorf("Confirm(%q) = false, want true", strings.TrimSpace(input))
				}
			})
		})
	}
}

func TestConfirm_No(t *testing.T) {
	for _, input := range []string{"n\n", "N\n", "no\n", "anything\n"} {
		t.Run(strings.TrimSpace(input), func(t *testing.T) {
			withStdin(t, input, func() {
				got, err := Confirm(false)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got {
					t.Errorf("Confirm(%q) = true, want false", strings.TrimSpace(input))
				}
			})
		})
	}
}

func TestConfirm_DefaultYes(t *testing.T) {
	withStdin(t, "\n", func() {
		got, err := Confirm(true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got {
			t.Error("Confirm with defaultYes=true and empty input should return true")
		}
	})
}

func TestConfirm_DefaultNo(t *testing.T) {
	withStdin(t, "\n", func() {
		got, err := Confirm(false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got {
			t.Error("Confirm with defaultYes=false and empty input should return false")
		}
	})
}

func TestConsumeUTF8Continuation(t *testing.T) {
	// 2-byte: 0xC3 0xA9 (e with accent)
	withStdin(t, "\xA9", func() {
		consumeUTF8Continuation(0xC3)
	})

	// 3-byte: 0xE2 0x80 0x99 (right single quotation mark)
	withStdin(t, "\x80\x99", func() {
		consumeUTF8Continuation(0xE2)
	})

	// 4-byte: 0xF0 0x9F 0x98 0x80 (grinning face emoji)
	withStdin(t, "\x9F\x98\x80", func() {
		consumeUTF8Continuation(0xF0)
	})
}
