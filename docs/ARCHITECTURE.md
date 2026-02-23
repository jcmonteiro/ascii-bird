# Architecture

> A terminal Flappy Bird in one file. Here's how the sausage is made.

## Overview

```
main.go (~1089 lines)
├── Terminal helpers      Raw mode, cursor, ANSI colors
├── Game constants        Physics, pipe geometry, timing
├── Data types            Bird, Pipe, Game, GameState
├── Game lifecycle        NewGame, startGame, die, resetBird
├── Pipe system           spawnInitialPipes, makePipe, pipeScreenX
├── Input                 readInput goroutine, InputEvent channel
├── Update                Physics step, scoring, collision, pipe recycling
├── Rendering             clearBuf, render*, differential render()
├── Clouds                Background decoration system
└── Main loop             Ticker-driven frame loop with physics accumulator
```

## The Three Loops

The game runs three decoupled systems at independent rates:

### 1. Input Loop (goroutine, ~1ms poll)

```
readInput goroutine
    └── os.Stdin.Read() with VMIN=0, VTIME=0
        ├── Got byte → parse → send InputEvent to channel
        └── Got nothing → time.Sleep(1ms) → retry
```

Runs in its own goroutine. Reads stdin in non-blocking mode (termios `VMIN=0, VTIME=0`). Parses single bytes and escape sequences (arrow keys). Sends `InputFlap` or `InputQuit` events into a buffered channel.

**Key detail:** The 1ms sleep on empty read prevents a CPU-burning busy-wait. Without it, `VMIN=0` causes `Read()` to return immediately with 0 bytes, spinning the CPU to 100%.

### 2. Physics Loop (30 Hz fixed step)

```
physicsAccum += elapsed
while physicsAccum >= 33ms:
    physicsAccum -= 33ms
    update(pendingFlap)      ← fixed-rate game logic
    pendingFlap = false      ← consumed on first tick
```

Runs inside the render loop using a **time accumulator**. Each `update()` call is exactly one 33ms physics step, regardless of how much wall time has passed. This makes game behavior deterministic and frame-rate-independent.

**Key detail:** `pendingFlap` is a persistent flag, not a per-frame variable. If the render loop runs a frame where no physics step executes (because not enough time has accumulated), the flap input is **preserved** until the next physics tick. Without this, inputs get silently dropped.

### 3. Render Loop (~60 FPS ticker)

```
every 16ms:
    drain input channel → set pendingFlap/pendingQuit
    run 0..N physics steps (accumulator)
    compute renderAlpha for interpolation
    compose frame into buf/colBuf
    diff against prevBuf/prevColBuf
    emit only changed cells to stdout
```

Drives everything. The 16ms ticker (~60 FPS) provides responsive input polling while the differential renderer keeps actual PTY output minimal.

## Differential Rendering

The single most important optimization. Without it, every frame writes ~48KB of ANSI codes to stdout. With it, a typical frame during gameplay writes ~1KB.

### How It Works

```
Game struct:
    buf / colBuf         ← "back buffer" — frame being composed
    prevBuf / prevColBuf ← "front buffer" — what's on screen
    fullRedraw           ← force all cells dirty
```

1. Game logic writes into `buf`/`colBuf` (the back buffer)
2. `render()` iterates every cell and compares against `prevBuf`/`prevColBuf`
3. **Changed cells only** get: cursor position (`\033[r;cH`) + color + character + reset
4. Changed cells are copied to the front buffer
5. `fullRedraw` flag is cleared

### When Full Redraws Happen

- First frame after `allocBuffers()` (front buffer is zeroed)
- `startGame()` — state transition from title/dead to playing
- `die()` — need to draw the game-over overlay cleanly

### Measured Impact

| Scenario | Full Render | Diff Render | Reduction |
|---|---|---|---|
| Identical frame | 48KB | 0 bytes | 100% |
| Pipe moves 1 col | 48KB | 1.1KB | 98% |
| Single cell change | 48KB | <100 bytes | 99.8% |

## Pipe System

### Lifecycle

```
spawnInitialPipes()
    → creates (width/spacing)+3 pipes starting at width+10
    → each spaced exactly pipeSpacing apart

Per physics tick:
    scrollOffset += pipeSpeed          ← float64, sub-pixel precision
    pipeScreenX(p) = p.x - round(scrollOffset + pipeSpeed*renderAlpha)

    if first pipe screen X < -(pipeWidth+2):
        remove from front
        append new pipe at lastPipe.x + pipeSpacing
```

### Why Float64 Scroll Offset

Pipes don't move by decrementing `p.x` each tick. Instead, each pipe has a fixed base `x` position, and a global `scrollOffset` float64 accumulates sub-pixel movement. `pipeScreenX()` computes the screen position on demand.

This eliminates the integer-based "move/no-move" pattern that caused visible vibration: with integer scroll, a pipe would stay still for 1 frame then jump 1 pixel, creating an uneven cadence.

### Render Interpolation

Between physics ticks, `renderAlpha` (0.0–1.0) represents how far we are between steps. `pipeScreenX()` adds `pipeSpeed * renderAlpha` to the scroll offset for sub-tick positioning. This smooths the visual motion when render rate > physics rate.

### Pool Sizing and Removal

The pipe pool size is `(width / pipeSpacing) + 3`. The extra 3 ensure there's always a pipe buffered off-screen on each side, preventing visible pop-in (right edge) or pop-out (left edge).

Removal threshold is `x < -(pipeWidth + 2)`, not `x < 0`. The `+2` accounts for pipe caps that extend 1 character beyond the pipe body on each side.

## Collision Detection

AABB (Axis-Aligned Bounding Box). The bird hitbox is 3 wide × 2 tall, centered on `(bird.x, bird.y)`.

```
birdLeft  = bird.x - 1      birdRight  = bird.x + 1
birdTop   = int(bird.y)      birdBottom = int(bird.y) + 1
```

Collision uses `int()` truncation for the bird row, **not** `math.Round()`. The visual bird position uses `math.Round()` for smoother animation at the arc apex. These are intentionally decoupled — visual smoothness shouldn't affect gameplay fairness.

## Terminal Management

### Raw Mode

Enabled via `ioctl` (`TIOCGETA` / `TIOCSETA`) with:
- Input: no break processing, no CR→NL, no parity, no strip, no flow control
- Output: no post-processing (`OPOST` off)
- Local: no echo, no canonical mode, no extended processing, no signals
- `VMIN=0, VTIME=0`: non-blocking reads (return immediately with 0 bytes if nothing available)

### Escape Sequence Parsing

Arrow keys arrive as 3-byte sequences: `ESC [ A` (up), `ESC [ B` (down), etc. To distinguish standalone ESC (quit) from an escape sequence prefix:

1. Read `ESC` byte
2. `time.Sleep(25ms)` — wait for remaining bytes
3. Non-blocking read — if nothing follows, it was standalone ESC
4. If `[` follows, read one more byte to identify the specific key

This 25ms delay is a pragmatic tradeoff. Shorter delays risk missing sequence bytes; longer delays make ESC-to-quit feel sluggish.

### Clean Shutdown

- Original termios saved on startup, restored on exit
- Signal handler for SIGINT/SIGTERM restores terminal state
- Cursor shown/hidden via `\033[?25l` / `\033[?25h`
- `clearScreen()` on exit so the terminal isn't left with game artifacts

## File Structure

```
ascii-bird/
├── main.go          Game source (1089 lines)
├── main_test.go     Test suite (2039 lines, 99 tests)
├── go.mod           Module definition
├── go.sum           Dependency checksums
├── .gitignore       Binary and OS artifacts
├── README.md        Public-facing docs with snide humor
└── docs/
    ├── ARCHITECTURE.md   This file
    ├── DISCOVERIES.md    Hard-won lessons and gotchas
    └── TUNING.md         Physics constants and their rationale
```
