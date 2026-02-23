# AGENTS.md — ASCII Bird

Guidelines for AI coding agents operating in this repository.

## Build / Run / Test

```bash
# Build
go build -o ascii-bird .

# Run
./ascii-bird

# Run all tests
go test -v ./...

# Run a single test by name (regex match)
go test -v -run TestCollision_Ground ./...

# Run a category of tests (regex prefix)
go test -v -run TestPipeRecycling ./...

# Race detector (catches concurrency bugs in the input goroutine)
go test -race ./...

# Quick build check (compile without producing binary)
go build ./...
```

There is no linter configured. Use `go vet ./...` for static analysis.

The LSP may report a false-positive `could not import golang.org/x/sys/unix` — ignore it, the code builds and tests fine. Go is installed at `/opt/homebrew/bin/go`.

## Project Layout

This is a single-package project. All game code lives in `main.go` (~1089 lines) and all tests live in `main_test.go` (~2039 lines, 99 tests). There is no `cmd/`, `pkg/`, or `internal/` structure — don't introduce one.

```
main.go          # Game source: terminal, physics, rendering, input, game loop
main_test.go     # Test suite: 20 numbered categories, 99 tests
go.mod / go.sum  # Module: github.com/ascii-bird, dep: golang.org/x/sys
docs/            # Architecture, discoveries, tuning docs (read these before deep changes)
```

## Code Style

### Formatting & Imports

- Run `gofmt` (or let the editor handle it). No custom format rules.
- Standard library imports come first, then a blank line, then third-party (`golang.org/x/sys/unix`).
- Only one third-party dependency exists. Do not add dependencies without explicit approval.

### Types & Naming

- Game state is represented by the `GameState` int enum (`StateTitle`, `StatePlaying`, `StateDead`).
- Structs: `Game`, `Bird`, `Pipe`, `Cloud`. Keep them flat — no nested structs or interfaces.
- Physics constants are package-level `const` values (e.g., `gravity`, `flapStrength`, `pipeGap`). Do not turn these into Game fields unless there's a reason to vary them at runtime.
- Color palette variables are package-level `var` values prefixed with `col` (e.g., `colSky`, `colPipe`).
- Helper functions on `*Game` are methods. Free functions (e.g., `farewellMessage`, `scrollClouds`) are used when they don't need Game state.

### Error Handling

- Terminal setup (`enableRawMode`) panics on error — this is intentional. The game cannot run without raw mode.
- Everywhere else, errors from `os.Stdin.Read` are silently ignored (non-blocking reads return 0 bytes routinely).
- Do not introduce `log` or structured logging. This is a terminal game; stdout is the screen.

### Rendering

- Double-buffered differential rendering: `buf`/`colBuf` (back) vs `prevBuf`/`prevColBuf` (front).
- `render()` only emits ANSI escape sequences for cells that changed since the previous frame.
- `fullRedraw` must be set to `true` on every state transition (`startGame()`, `die()`) and on first frame. Forgetting this causes visual garbage.
- Visual bird position uses `math.Round()`. Collision bird position uses `int()` truncation. They are intentionally decoupled.

### Physics & Game Loop

- Physics runs at a fixed 30 Hz (`physicsRate = 33ms`). Render runs at ~60 FPS (`renderRate = 16ms`).
- Input flags (`pendingFlap`, `pendingQuit`) persist across render frames to avoid dropped inputs when no physics step runs in a given render frame.
- Pipe positions use `float64 scrollOffset` with `pipeScreenX()` for smooth sub-pixel scrolling. Never move pipes by decrementing integer `x` directly in the game loop.

## Test Conventions

### Structure

- Tests are grouped into 20 numbered categories with `═══` section headers.
- `testGame()` creates an 80x24 game via `newGameWithSize(w, h)` — no real terminal needed.
- `bufText()` and `bufRow(n)` extract plain text from the character buffer for visual assertions.
- When testing pipe positions, set `g.scrollOffset` and `g.renderAlpha` explicitly or account for `pipeScreenX()` applying them.

### Style

- Use `t.Errorf` for non-fatal assertions, `t.Fatalf` only when subsequent assertions would be meaningless.
- Use `t.Logf` for informational output (e.g., "bird died at y=12.3").
- Float comparisons must use epsilon (`eps := 0.001`) — never `==` or `!=` on accumulated gravity values.
- Table-driven tests for medal tiers, terminal sizes, and similar parameterized cases.

### Adding Tests

- Place new tests in the appropriate numbered category, or create a new category with the next number.
- Follow TDD: write a failing test first, verify it catches the bug, then fix and confirm it passes.
- Run `go test -v -run TestYourNewTest ./...` to verify in isolation before running the full suite.

## Commit Conventions

- Use [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `test:`, `refactor:`, `docs:`, `chore:`.
- Commit messages should be concise (1-2 sentences) and focus on "why" not "what".
- Do not commit the `ascii-bird` binary or `.DS_Store`.

## Humor Policy

- All user-facing text (farewell messages, README, docs) should be "edgy with a snide sense of humor."
- Never offensive, just sarcastically self-aware.
- Farewell messages are randomized — each score tier has 3-4 variants. Maintain this pattern when adding new tiers.
