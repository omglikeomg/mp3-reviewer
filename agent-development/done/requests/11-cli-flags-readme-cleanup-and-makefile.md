# Task 11: CLI Flags, README Cleanup & Makefile

## Goal

Three small, independent operational improvements: (1) add `--help` and `--version` flags to `cmd/reviewer/main.go`, (2) correct the README's inaccurate FLAC claim, and (3) add a `Makefile` with standard `build`, `test`, `install`, and `lint` targets.

## Context

Requires Tasks 0–10 to be completed first.

**Why these three together?** They are all pure operational/packaging concerns that touch no domain logic and share no dependencies. Each is low-risk, fully self-contained, and closes specific gaps identified in the Task 10 state-of-development review (points 5, 7, and 8).

### Problem 1 — No `--help` or `--version` flags

`cmd/reviewer/main.go` currently calls `tui.New()` unconditionally on startup regardless of what arguments are passed. A user running `./song-reviewer --help` gets the TUI, not usage text. A user wanting to confirm which binary version they have installed has no way to do so. Both flags are expected by any command-line tool and are trivially cheap to add.

`--version` should print a version string injected at build time via `-ldflags`. If the binary is built without the flag (e.g. `go run`), it falls back to `"dev"`.

`--help` should print a concise usage block describing what the tool does, how to configure it, and the key runtime keybindings — enough for a new user to get started without reading the full README.

### Problem 2 — README claims FLAC support that doesn't exist

The README Features section says "MP3/FLAC ID3 tags" and the `internal/metadata` package description says "MP3/FLAC". The `beep` library has separate FLAC decoding paths, `bogem/id3v2` is an ID3-specific library and does not handle FLAC's Vorbis comment metadata at all, and no FLAC code path has been written or tested. This claim is inaccurate and would mislead users who try to load FLAC files. The fix is to remove the FLAC references from the README and update the architecture docs to reflect MP3-only scope.

### Problem 3 — No Makefile

Users and contributors must remember raw `go` commands to build, test, and install. A minimal `Makefile` with four targets makes the project easier to work with and documents the standard workflow in one place.

## Requirements

### CLI flags

- `cmd/reviewer/main.go` must inspect `os.Args` **before** constructing any TUI or audio state.
- `--help` (also `-h`) prints a usage message to stdout and exits with code 0. It must not start the TUI.
- `--version` (also `-v`) prints the version string to stdout and exits with code 0. It must not start the TUI.
- The version string is controlled by a package-level `var version = "dev"` in `main.go`. When building with `go build -ldflags "-X main.version=1.2.3"`, the injected value is used; otherwise `"dev"` is printed.
- Any unrecognised flag (e.g. `--foo`) prints a short error to stderr (`song-reviewer: unknown flag: --foo\nRun 'song-reviewer --help' for usage.`) and exits with code 1. It must not start the TUI.
- If no flags are passed (the normal case), startup proceeds exactly as today — no change in behaviour.
- Use only the standard library (`os`, `fmt`). Do **not** use `flag` package or any third-party flag parser. A simple `switch os.Args[1]` over the first argument is sufficient.

### `--help` output format

The help text must include:
- A one-line description of the tool.
- A usage line: `Usage: song-reviewer [--help] [--version]`
- A "Configuration" section pointing to `config/settings.json` and `settings.example.json`.
- A "Keybindings" section listing the primary keys (seek, pause, tag, BPM tap, commit BPM/Year, skip, undo, settings, quit) — these can be condensed from the README table.
- A trailing note that the full README has more detail.

### README correction

- Remove all references to FLAC from the README. Specifically:
  - The Features bullet that says "MP3/FLAC" must be changed to "MP3".
  - Any other FLAC mention in the README (e.g. in the Persistence section or architecture table) must be corrected to MP3-only.
- Do **not** add any explanation of "why no FLAC" — simply state MP3 where FLAC was previously mentioned.

### Architecture doc correction

- In `agent-development/agent-specs/architecture-breakdown.md`, the `/internal/metadata` entry currently says "MP3/FLAC". Correct it to MP3-only to match the README correction.

### Makefile

- Located at the project root: `Makefile`.
- Four targets: `build`, `test`, `install`, `lint`.
- `build`: runs `go build -ldflags "-X main.version=$(VERSION)" -o song-reviewer ./cmd/reviewer`. `VERSION` defaults to `dev` if not set (use `VERSION ?= dev`).
- `test`: runs `go test ./...`.
- `install`: runs `go install -ldflags "-X main.version=$(VERSION)" ./cmd/reviewer`.
- `lint`: runs `go vet ./...`. If `staticcheck` is available on `$PATH` it is also run; if not, `go vet` alone is sufficient (do not make `staticcheck` a hard dependency).
- A `.PHONY` declaration covers all four targets.
- Include a brief comment header at the top of the `Makefile` explaining the available targets.

## Implementation Details

1. **`cmd/reviewer/main.go`:**
   - Add `var version = "dev"` at package level (before `func main()`).
   - At the very top of `main()`, before any other logic, add:
     ```go
     if len(os.Args) > 1 {
         switch os.Args[1] {
         case "--help", "-h":
             fmt.Print(helpText)
             os.Exit(0)
         case "--version", "-v":
             fmt.Println("song-reviewer", version)
             os.Exit(0)
         default:
             fmt.Fprintf(os.Stderr, "song-reviewer: unknown flag: %s\nRun 'song-reviewer --help' for usage.\n", os.Args[1])
             os.Exit(1)
         }
     }
     ```
   - Define `helpText` as a package-level `const string` (not a `var`) so it is a compile-time constant. Write it using a raw string literal (backtick-delimited) for readability.
   - The `helpText` constant must be defined in the same file (`main.go`) — do not create a separate file for it.

2. **`Makefile`** (project root):
   - Use tabs (not spaces) for recipe indentation — `make` requires this.
   - The `lint` target should use a shell conditional to optionally run `staticcheck`:
     ```makefile
     lint:
     	go vet ./...
     	@if command -v staticcheck > /dev/null 2>&1; then staticcheck ./...; fi
     ```

3. **`README.md`:**
   - Search for every occurrence of "FLAC" (case-sensitive) and replace or remove as described in the requirements above. There are approximately 2–3 occurrences.

4. **`agent-development/agent-specs/architecture-breakdown.md`:**
   - Search for "MP3/FLAC" and correct to "MP3".

## Deliverables

- [ ] `var version = "dev"` added to `cmd/reviewer/main.go`.
- [ ] `--help` / `-h` flag: prints usage and exits 0 without starting the TUI.
- [ ] `--version` / `-v` flag: prints `song-reviewer <version>` and exits 0 without starting the TUI.
- [ ] Unknown flags print an error to stderr and exit 1.
- [ ] `helpText` const defined in `main.go` covering description, usage, configuration, and keybindings.
- [ ] `Makefile` at project root with `build`, `test`, `install`, `lint` targets and `VERSION ?= dev`.
- [ ] `go build -ldflags "-X main.version=1.2.3" ./cmd/reviewer && ./song-reviewer --version` prints `song-reviewer 1.2.3`.
- [ ] All FLAC references removed from `README.md`.
- [ ] `agent-development/agent-specs/architecture-breakdown.md` corrected to MP3-only.
- [ ] `agent-development/agent-specs/FOLDER-STRUCTURE.md` updated to include `Makefile` at the project root.

## Agent Checklist

- [ ] `go build ./...` succeeds with zero errors.
- [ ] `go vet ./...` exits 0 with no output.
- [ ] `go test ./...` passes with zero failures (no existing tests must break).
- [ ] `./song-reviewer --help` prints usage text and exits 0 (verify by inspection).
- [ ] `./song-reviewer -h` prints usage text and exits 0.
- [ ] `./song-reviewer --version` prints `song-reviewer dev` (or the injected version) and exits 0.
- [ ] `./song-reviewer --unknown` prints an error to stderr and exits with code 1.
- [ ] `go build -ldflags "-X main.version=1.2.3" -o song-reviewer ./cmd/reviewer && ./song-reviewer --version` prints `song-reviewer 1.2.3`.
- [ ] `make build` produces a `song-reviewer` binary in the project root.
- [ ] `make test` runs the full test suite.
- [ ] `make lint` runs `go vet ./...` without error.
- [ ] `grep -i flac README.md` returns no matches.
- [ ] `grep -i flac agent-development/agent-specs/architecture-breakdown.md` returns no matches.
- [ ] No unrelated files were modified.
- [ ] Update `agent-development/agent-specs/FOLDER-STRUCTURE.md` to include the `Makefile` entry.
- [ ] Update `README.md` — add a "Development" or "Build" section near the Installation section that shows `make build`, `make test`, `make install`, and `make lint` as the canonical commands.