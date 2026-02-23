# ASCII Bird

```
                                                   ██  ██
                                                   ██  ██
                                                   ██  ██
      ●▶         ●▶         ●▶          ●▶         ██  ██
     ╱█▶  ...   ▄█▶   ...  ╱█▶   ...   ▄█▶       ██████████
                                                   ██  ██
                                                   ██  ██
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~██~~██~~~~~~~
```

A Flappy Bird clone that runs in your terminal. No pixels. No frameworks. Just ASCII characters, ANSI escape codes, and a few lines of Go that I am mass-overqualifying with tests and architecture documents.

## Features Nobody Asked For

- **Bird physics** — Gravity pulls you down. Flapping pushes you up. Revolutionary.
- **Procedurally generated pipes** — Random gaps so I can blame RNG instead of my reflexes
- **Differential rendering** — Double-buffered, diffed cell-by-cell. Cuts output from ~48KB to ~1KB per frame. Your PTY thanks me.
- **Title screen with animated bird** — It bobs. Its wings flap.
- **Scrolling clouds** — Atmospheric. Decorative. Completely pointless.
- **Score tracking with medals** — Bronze, Silver, Gold, Platinum. I have seen Bronze once.
- **Best score persistence** — Per session only, because saving to disk would imply I'll play this more than once
- **256-color rendering** — The terminal supports 16 million colors and I use maybe 12 of them
- **Raw terminal mode** — I hijack your terminal at the syscall level via `ioctl`. You're welcome.
- **Signal handling** — Ctrl+C won't leave your terminal broken. I'm not an animal.

## Getting Started

```bash
git clone https://github.com/jcmonteiro/ascii-bird.git
cd ascii-bird
go build -o ascii-bird .
./ascii-bird
```

That's it. No flags. No config files. `go build` is faster than opening Unity and pretending I'll finish my RPG

**Requirements:** Go 1.21+, macOS/Linux (I use `golang.org/x/sys/unix` — Windows users, WSL is your friend), and a terminal emulator that isn't from 1987.

## Tuning

| Constant       | Value          | Why                                                      |
| -------------- | -------------- | -------------------------------------------------------- |
| Gravity        | 0.18           | Any lower and the bird floats like a helium balloon      |
| Flap strength  | -1.35          | Negative because up is negative in screen coords.        |
| Max fall speed | 2.8            | Terminal velocity — prevents unrecoverable death spirals |
| Pipe width     | 6 chars        | Wide enough to render, narrow enough to dodge            |
| Pipe gap       | 11 rows        | Generous. You'll still die.                              |
| Pipe spacing   | 25 cols        | Just enough time to regret your last flap                |
| Pipe speed     | 0.67 cols/tick | Sub-pixel, float64-based. Pipes don't jitter.            |
| Physics rate   | 33ms (30 Hz)   | Fixed step. Deterministic.                               |
| Render rate    | 16ms (~60 FPS) | Differential rendering keeps output under 1KB/frame      |

Full tuning rationale in [`docs/TUNING.md`](docs/TUNING.md).

---

_Not affiliated with or endorsed by any bird-related mobile game franchise, past or present._
