# Known Issues

Issues worth fixing that didn't fit the branch they were discovered in.

## Two bufio.NewReaders wrapping the same os.Stdin

`Init()` creates a `bufio.NewReader(os.Stdin)` for the "Continue?" prompt, then `setupRepo()` creates a second one. In interactive use this works because terminal line buffering means the first reader only consumes one line. But piped input or fast typing could cause the first reader to swallow bytes meant for the second.

**Fix:** Pass the reader from `Init()` into `setupRepo()` as a parameter so a single reader serves all prompts. This also fixes the `initRepoWithMobsDir` test workaround - tests could provide all answers in one stdin string and have them actually reach the right prompts.

**Files:** `internal/mob/init.go:186`, `internal/mob/init.go:635`, `internal/mob/integration_test.go:85-135`
