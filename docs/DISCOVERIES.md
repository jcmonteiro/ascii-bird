# Discoveries

> Things that took longer to figure out than they should have. Future AI sessions: read this before you touch anything.

## Terminal I/O

### VMIN=0 Causes CPU Spin

**Problem:** Setting termios `VMIN=0, VTIME=0` makes `os.Stdin.Read()` return immediately with 0 bytes when no input is available. Putting this in a goroutine loop burns 100% of one CPU core.

**Fix:** Add `time.Sleep(1 * time.Millisecond)` when `Read()` returns 0 bytes. This yields the CPU while keeping input latency imperceptible.

**Don't:** Use `VMIN=1` (blocking read) — it blocks the goroutine and you lose the ability to check a quit channel.

### os.Stdin.SetReadDeadline() Doesn't Work

`os.Stdin` is a regular file descriptor, not a network connection. `SetReadDeadline()` only works on types that implement `net.Conn`. For terminal input with timeouts, you must use termios `VMIN`/`VTIME` settings or a manual sleep-and-retry loop.

### Escape Sequence Parsing Needs a Sleep

Arrow keys arrive as multi-byte sequences (`ESC [ A` for up arrow). The bytes don't always arrive in a single `Read()` call. After reading `ESC` (0x1B), you need to wait ~25ms then do a non-blocking read to check for continuation bytes.

- Too short (< 10ms): miss the `[A` bytes, misinterpret as standalone ESC
- Too long (> 50ms): ESC key feels sluggish to the user
- 25ms is the sweet spot most terminal libraries use

## Rendering

### Differential Rendering is Non-Negotiable

The single biggest improvement to visual quality. Rewriting the entire 80x24 screen every frame at 60 FPS pushes ~2.8MB/sec through the PTY. Terminal emulators can handle this, but the interpretation and rendering lag causes:

- Visible tearing/flicker as partial frames display
- Pipe "vibration" where characters appear to jiggle
- General visual noise that makes the game look broken

**Solution:** Double-buffer (front/back) with cell-by-cell comparison. Only emit ANSI codes for changed cells. A typical gameplay frame changes ~50-100 cells out of 1920, reducing output by 95-98%.

**Important:** Force a full redraw (`fullRedraw = true`) on state transitions (start game, die, restart). The overlay and scene composition change dramatically, and the diff against the previous state's frame would produce garbage.

### math.Round() vs int() — Visual vs Collision

The bird's visual row uses `math.Round(bird.y)` for smooth animation at the apex of a flap arc (where velocity passes through zero). Collision detection uses `int(bird.y)` (truncation). These are intentionally different:

- `Round()` makes the bird visually "hover" at the top of an arc for an extra frame — feels smooth
- `int()` is conservative for collision — the bird doesn't clip into pipes it shouldn't
- If you unify them, either the animation looks jerky or collisions feel unfair

### Pipe Position Rendering Must Be Float-Based

Early implementation moved pipes by decrementing `pipe.x` each physics tick. With `pipeSpeed=0.67`, this becomes: advance 67% of the time, stay 33%. The resulting move/no-move pattern is **visually obvious** as a stuttery vibration.

**Fix:** Pipes have fixed base `x` positions. A global `scrollOffset` (float64) accumulates movement. `pipeScreenX()` computes the display position by subtracting `math.Round(scrollOffset)` from the base position. This distributes the sub-pixel error smoothly.

**Further fix:** Render-time interpolation via `renderAlpha` adds `pipeSpeed * alpha` to the offset for even smoother inter-tick positioning.

### Per-Cell Cursor Positioning vs Row-Based

The old renderer wrote each row left-to-right with a single `\033[row;1H` cursor position per row, then characters sequentially. The differential renderer positions each changed cell individually with `\033[row;colH`. This uses more bytes per cell but **far fewer total bytes** because most cells don't change.

For a full redraw, the per-cell approach is ~10% more expensive than row-based. But full redraws only happen on state transitions (~once per game session), so this tradeoff is overwhelmingly positive.

## Physics

### Decoupled Physics is Essential

Physics **must** run at a fixed rate independent of render rate. If physics runs per-render-frame:

- At 60 FPS: gravity accumulates 60x per second → bird falls too fast
- At 30 FPS: gravity accumulates 30x per second → bird floats
- Frame drops cause gameplay speed changes

**Solution:** Time accumulator pattern. Track elapsed time since last physics step. Run `update()` in a `while (accum >= physicsRate)` loop. The game behaves identically whether rendering at 30, 60, or 144 FPS.

### Input Flags Must Persist Across Frames

When physics runs at 30Hz and rendering at 60Hz, roughly half of render frames execute zero physics steps. If you drain the input channel into a frame-local `flap` variable, then no physics step runs, the flap is **silently dropped**.

**Fix:** Use persistent `pendingFlap` / `pendingQuit` flags on the game loop scope. Set them when draining input. Clear them only when a physics step consumes them. This guarantees every input reaches the physics system.

### Terminal Velocity Cap

Without `maxFallSpeed`, a bird that falls from the ceiling accumulates `vy = gravity * frames` and hits the ground at extreme velocity. The next flap barely dents this speed, making recovery from long falls feel impossible.

`maxFallSpeed = 2.8` caps downward velocity. This means a flap from terminal velocity reliably produces upward movement, keeping the game fair.

## Pipe Management

### Pool Size Must Include Off-Screen Buffer

If you only spawn enough pipes to fill the visible screen, pipes appear to "pop in" at the right edge (they appear as a fully-formed column in one frame). Similarly, pipes "pop out" at the left edge if removed too early.

**Fix:** Pool size is `(width / pipeSpacing) + 3`. The extra pipes ensure there's always at least one buffered off each side of the viewport. New pipes are spawned at `lastPipe.x + pipeSpacing`, which is always off-screen to the right.

### Cap-Aware Removal Threshold

Pipe caps extend 1 character beyond the pipe body on each side. If you remove a pipe when `screenX < -pipeWidth`, the left cap (at `screenX - 1`) may still be visible at column -1... which isn't visible. But at `screenX = -(pipeWidth)`, the right cap is at `screenX + pipeWidth`, which wraps to 0 — visible!

Actually, the issue is simpler: at `screenX = -(pipeWidth + 1)`, the left cap is at `-(pipeWidth + 2)`, safely off-screen. The removal threshold `x < -(pipeWidth + 2)` ensures no visual artifact from caps.

### Never Clamp Recycled Pipe Positions

Early code had `if newX < width+2 { newX = width+2 }` to ensure new pipes spawn off-screen. This **breaks even spacing** when the last pipe is near the viewport edge, because the clamped position doesn't maintain the `pipeSpacing` interval.

**Fix:** Always set `newX = lastPipe.x + pipeSpacing`. The pool sizing guarantees this is off-screen. If it somehow isn't, that's a pool size bug, not something to paper over with a clamp.

## Testing

### Floating-Point Comparison Needs Epsilon

Gravity accumulation produces values like `0.18, 0.36, 0.54...` which are not exactly representable in float64. Comparing with `!=` fails randomly. Use `eps := 0.001` and check `abs(actual - expected) < eps`.

### newGameWithSize() for Test Isolation

Tests can't call `NewGame()` because it calls `getTermSize()` which does an `ioctl` on stdout — there's no terminal in a test runner. `newGameWithSize(w, h)` was extracted specifically for tests. Always use `testGame()` (which calls `newGameWithSize(80, 24)`) in tests.

### Render Tests Need fullRedraw Awareness

After the differential rendering change, `render()` only emits changed cells. Tests that call `render()` and check the output for specific content must be aware that:

- First render on a new game: `fullRedraw=true`, all cells emitted (tests pass naturally)
- Second render with same buffer: output is empty (tests checking ANSI codes would fail)

Most existing tests only render once per test case and create fresh games, so they naturally get a full first render. But if you add tests that render multiple times, be aware of this.

## Go-Specific

### bytes.Buffer vs strings.Builder for Reuse

`strings.Builder.Reset()` sets the internal buffer to `nil`, releasing the allocation. Next use allocates fresh memory.

`bytes.Buffer.Reset()` sets `buf = buf[:0]`, retaining the allocated capacity. Next use reuses the same memory.

For a render buffer that's written and reset 60 times per second, `bytes.Buffer` eliminates ~60 allocations/sec. The `renderBuf` on the `Game` struct uses `bytes.Buffer` for this reason.

### LSP False Positives for golang.org/x/sys/unix

The Go language server repeatedly reports "could not import golang.org/x/sys/unix" on the `import` line. This is a false positive. The package is in `go.sum`, `go build` works fine, and `go test` passes. Ignore this diagnostic.

### rand.Seed Deprecation

`rand.Seed()` is deprecated in Go 1.20+ (the global PRNG auto-seeds). We use it anyway for backward compatibility with Go 1.21 minimum requirement. It produces a vet warning but no functional issue.
