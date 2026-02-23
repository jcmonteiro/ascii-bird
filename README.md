# ASCII Bird

> Because you definitely needed *another* Flappy Bird clone. But this one runs in your terminal, so at least you can pretend you're working.

```
        ___
    ___/   \___        ██  ██
   |  ██  ●    >      ██  ██
   |__~~_/____/        ██  ██
                       ██  ██
                     ████████████
                       ██  ██
                       ██  ██
~~~~~~~~~~~~~~~~~~~~~~~██~~██~~~~~~~
```

## What Is This

A pixel-perfect\* recreation of Flappy Bird — the game a solo dev made in a weekend that grossed $50k/day while your startup burns VC money on Kubernetes configs. Except this version has no pixels. It's all ASCII characters, ANSI escape codes, and poor life choices rendered at a buttery 60 FPS in your terminal emulator.

\*Pixel-perfect if you squint and have low standards.

## Features Nobody Asked For

- **Authentic bird physics** — Gravity pulls you down. Flapping pushes you up. Revolutionary.
- **Procedurally generated pipes** — Random gaps so you can blame RNG instead of your reflexes
- **Title screen with animated bird** — It bobs. Its wings flap. It's more alive than your social life.
- **Scrolling clouds** — Atmospheric. Decorative. Completely pointless.
- **Score tracking with medals** — Bronze, Silver, Gold, Platinum. You'll see Bronze once and never again.
- **Best score persistence** — Per session only, because saving to disk would imply you'll play this more than once
- **Game over overlay** — A nice box that shows you the number you already knew was embarrassingly low
- **256-color rendering** — Your terminal supports 16 million colors and we use maybe 12 of them
- **Raw terminal mode** — We hijack your terminal at the syscall level. You're welcome.
- **Signal handling** — Ctrl+C won't leave your terminal in a broken state. We're not *animals*.

## Requirements

- **Go 1.21+** (we use `golang.org/x/sys/unix`, so macOS/Linux only — Windows users, just dual-boot like a normal person)
- A terminal emulator that isn't from 1987
- The willingness to mass-close browser tabs and actually do something fun for 30 seconds

## Installation

```bash
git clone https://github.com/ascii-bird/ascii-bird.git  # or however you got here
cd ascii-bird
go build -o ascii-bird .
```

Congratulations. You just compiled 1,089 lines of Go to play a game a 7-year-old mastered on their iPad in 2014.

## Usage

```bash
./ascii-bird
```

That's it. No flags. No config files. No YAML. Refreshing, isn't it?

## Controls

| Key | Action |
|-----|--------|
| `Space` | Flap (or start game, or retry after dying — it's the universal "do something" button) |
| `Up Arrow` | Flap (for people who think Space is beneath them) |
| `W` | Flap (for WASD purists who wandered in from an FPS) |
| `Q` | Quit during title/game over. Pauses to mock you during gameplay. |
| `Esc` | Quit (for the dramatic) |

## Medal Tiers

| Medal | Score | Difficulty |
|-------|-------|------------|
| None | 0-9 | You |
| Bronze | 10+ | Your non-gamer friend |
| Silver | 20+ | Getting suspicious |
| Gold | 30+ | Okay you're actually decent |
| Platinum | 40+ | Liar, or you modified the source code |

## Architecture

It's one file. `main.go`. 1,089 lines. No frameworks. No entity-component systems. No design patterns named after furniture.

Here's what's in there:

- **Terminal wrangling** — Raw mode via `ioctl`, ANSI escape codes for color/cursor, signal traps for clean shutdown
- **Differential rendering** — Double-buffered character + color grids. Each frame is diffed cell-by-cell against the previous frame; only changed cells emit ANSI codes. Cuts per-frame output from ~48KB to ~1KB. Your PTY thanks us.
- **Decoupled physics** — Fixed 30 Hz simulation step with time accumulator, independent of the 60 FPS render loop. Game feels identical regardless of frame rate.
- **Bird physics** — `velocity += gravity` every tick, `velocity = flapStrength` on input. Welcome to Newtonian mechanics, population: one yellow bird.
- **Pipe system** — Float64 scroll offset with render-time interpolation. Pipes glide instead of jittering. Spawned ahead of screen, recycled when they exit. Like a conveyor belt of death.
- **Collision detection** — AABB checks against pipes, ground, and ceiling. The ceiling kills you because the sky is not, in fact, the limit.
- **Input handling** — Non-blocking reads on stdin with escape sequence parsing for arrow keys. Polled at 60 Hz for responsive flaps. More fiddly than it has any right to be.

For the full technical deep-dive, see [`docs/`](docs/).

## Tests

Yes, there are tests. 99 of them. For a Flappy Bird clone. In a terminal.

```bash
go test -v ./...
```

```
ok  github.com/ascii-bird  0.228s   # 99/99 PASS
```

Test coverage includes:

- Game initialization and state transitions
- Bird physics (gravity, flap impulse, acceleration, terminal velocity)
- Collision detection (ground, ceiling, pipe edges, gap safety)
- Scoring and medal assignment
- Pipe generation, spacing, gap bounds, recycling, and pool sizing
- Pipe recycling regression tests (spacing stability, cap-aware removal, no pop-in/pop-out)
- Title screen rendering (title text, bird art, instructions, animation)
- Gameplay rendering (score display, bird sprite, pipe drawing, gap clarity)
- Game over overlay (text, medals, border, retry instructions)
- Bird and wing animations across frames
- Cloud initialization, scrolling, recycling, and z-ordering
- Buffer operations (clear, setCell bounds, drawCentered)
- ANSI color output verification
- **Differential rendering** (full redraw on first frame, zero output on identical frames, minimal output on single-cell changes, front buffer tracking, forced redraws on state transitions, output scaling proportional to visual delta)
- Small and large terminal size handling
- Edge cases (bird at boundaries, pipes at screen edges)
- Full game simulations (fall-to-death, AI-controlled flap survival, complete render cycles)

We test the *animations*. We have a simulated bird AI that flaps to survive 60 frames. We verify that moving a pipe 1 column produces proportionally small output. The test suite is more thorough than whatever QA process shipped Cyberpunk 2077.

## Technical Details for the Morbidly Curious

| Constant | Value | Why |
|----------|-------|-----|
| Gravity | 0.18 | Any lower and the bird floats like a helium balloon |
| Flap strength | -1.35 | Negative because up is negative in screen coords. Computer science is fun. |
| Max fall speed | 2.8 | Terminal velocity cap — prevents unrecoverable death spirals |
| Pipe width | 6 chars | Wide enough to render, narrow enough to dodge |
| Pipe gap | 11 rows | Generous. You'll still die. |
| Pipe spacing | 25 cols | Just enough time to regret your last flap |
| Pipe speed | 0.67 cols/tick | Sub-pixel, float64-based. Your pipes don't jitter. |
| Ground height | 3 rows | It's dirt. It doesn't need more. |
| Physics rate | 33ms (30 Hz) | Fixed step. Deterministic. Frame-rate-independent. |
| Render rate | 16ms (~60 FPS) | Differential rendering keeps output under 1KB/frame |

For the full tuning rationale, see [`docs/TUNING.md`](docs/TUNING.md).

## License

Do whatever you want with it. It's a terminal Flappy Bird. If you're reading this section hoping to commercialize it, we need to talk about your business plan.

## FAQ

**Q: Why?**
A: Why not?

**Q: No seriously, why?**
A: Because `go build` is faster than opening Unity and pretending you'll finish that RPG.

**Q: Does it work on Windows?**
A: It uses `unix.Termios` and `ioctl`. So no. Use WSL, or better yet, use a real OS.

**Q: Can I contribute?**
A: It's a single-file Flappy Bird clone with 99 tests and three architecture docs. What exactly would you add? Multiplayer? Loot boxes?

**Q: My terminal looks broken after playing.**
A: Did you `kill -9` the process? Signal handlers can't save you from yourself. Run `reset` in your terminal.

**Q: I got Platinum!**
A: Screenshot or it didn't happen.
