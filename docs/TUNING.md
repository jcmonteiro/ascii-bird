# Tuning Guide

> Every number in this game was argued about, tested, and changed at least twice. Here's why they are what they are.

## Physics Constants

| Constant | Value | Unit | Purpose |
|---|---|---|---|
| `gravity` | 0.18 | rows/tick^2 | Downward acceleration per physics tick |
| `flapStrength` | -1.35 | rows/tick | Upward velocity impulse on flap (negative = up) |
| `maxFallSpeed` | 2.8 | rows/tick | Terminal velocity cap — prevents punishing freefall |
| `pipeSpeed` | 0.67 | cols/tick | Horizontal scroll speed per physics tick |

### gravity = 0.18

At 30 Hz physics, this gives ~5.4 rows/sec^2 of acceleration. A bird in freefall from mid-screen (~row 10) reaches the ground (~row 20) in roughly 1.5 seconds. This feels urgent but not panicky.

**Too low (< 0.12):** Bird floats like a helium balloon. Game feels sluggish. Easy to over-flap into the ceiling.

**Too high (> 0.25):** Bird drops like a stone. Must spam flap constantly. Exhausting and frustrating.

### flapStrength = -1.35

A single flap gives ~4 rows of peak height gain (before gravity pulls back). This is enough to cross roughly 1/3 of the play area in one flap, which feels responsive without being overpowered.

**Why negative:** Screen coordinates have Y increasing downward. Negative velocity = upward movement.

**Too weak (> -1.0):** Flaps feel ineffective. Must double-flap constantly. Frustrating.

**Too strong (< -1.8):** Single flap overshoots most gaps. Ceiling collisions become the primary death cause. The gap size becomes irrelevant.

### maxFallSpeed = 2.8

Without this cap, a bird falling from the top of the screen accumulates `vy = 0.18 * frames`. After 30 frames (1 second), `vy = 5.4` — a single flap (`-1.35`) barely dents this, and the bird continues falling. This makes recovery from long falls feel impossible.

With `maxFallSpeed = 2.8`, a flap from terminal velocity brings `vy` to `-1.35` — reliably upward. This keeps the game fair: you can always recover if you react in time.

### pipeSpeed = 0.67

At 30 Hz, this gives ~20 columns/sec of scroll speed. A standard 80-column terminal scrolls across in ~4 seconds. With `pipeSpacing = 25`, a new pipe arrives roughly every 1.25 seconds.

**Too slow (< 0.4):** Game feels boring. Too much empty sky between pipes.

**Too fast (> 1.0):** Pipes arrive faster than human reaction time. The game becomes a twitch-fest with no room for strategy.

**Why 0.67 specifically:** It's 2/3 of a pixel per tick. This is the slowest speed that still felt "active" during play-testing. The sub-pixel nature (not an integer) is why float64 scroll offset and render interpolation are necessary.

## Pipe Geometry

| Constant | Value | Unit | Purpose |
|---|---|---|---|
| `pipeWidth` | 6 | columns | Horizontal thickness of pipe body |
| `pipeGap` | 11 | rows | Vertical opening between top and bottom pipes |
| `pipeSpacing` | 25 | columns | Horizontal distance between consecutive pipes |
| `groundHeight` | 3 | rows | Height of the ground area |

### pipeGap = 11

The bird hitbox is 3 wide × 2 tall. A gap of 11 gives 9 rows of clearance above and below the bird — about 4.5 bird-heights. This sounds generous, but the bird's vertical velocity makes threading the gap harder than the raw numbers suggest.

**History:** Started at 8. This was *brutal* — the bird barely fit through and any vertical velocity at the wrong moment meant death. Widened to 11 after play-testing, which transformed the game from "impossible" to "challenging but learnable."

**For different difficulties:** Gap 8-9 = hard mode, Gap 11-12 = normal, Gap 14+ = easy mode.

### pipeSpacing = 25

With `pipeSpeed = 0.67` at 30 Hz, pipes are ~1.25 seconds apart. This gives enough time to:
1. Recover from the previous pipe's gap (adjust velocity)
2. Read the next pipe's gap position
3. Plan and execute 2-3 flaps to reach it

At `pipeSpacing = 15`, pipes arrive every ~0.75 seconds — barely enough time to react. At `pipeSpacing = 35`, there's too much idle sky and the game drags.

### pipeWidth = 6

Narrow enough that pipes don't dominate the screen. Wide enough to render internal detail (edge characters `║`, body fill `█`, cap extensions `▄`/`▀`). The caps extend 1 character beyond each side, making the visual footprint 8 columns.

## Timing

| Constant | Value | Purpose |
|---|---|---|
| `physicsRate` | 33ms (30 Hz) | Fixed simulation step |
| `renderRate` | 16ms (~60 FPS) | Display refresh rate |

### physicsRate = 33ms (30 Hz)

Game logic runs at 30 steps per second. This is fast enough for smooth-feeling gameplay and slow enough that the physics constants (gravity, flapStrength) can be small, readable numbers.

**Why not 60 Hz:** Halving the physics rate would require halving all physics constants to maintain the same feel. The constants would become tiny and hard to reason about. 30 Hz is the standard for fixed-step game physics.

**Why not variable rate:** Variable physics makes the game non-deterministic. Two runs at different frame rates would feel different. Fixed step + accumulator ensures identical gameplay regardless of rendering speed.

### renderRate = 16ms (~60 FPS)

The render loop polls input and draws frames at ~60 FPS. This provides:

- **Responsive input:** Flap inputs are detected within 16ms
- **Smooth interpolation:** `renderAlpha` provides sub-tick pipe positioning between physics steps
- **Minimal output:** Differential rendering means most frames emit < 1KB even at this rate

**History:** Was originally 16ms, changed to 33ms when pipe vibration was bad (to match physics 1:1), which killed input responsiveness. Restored to 16ms after differential rendering eliminated the vibration.

## Play Area

For a standard 80×24 terminal:

```
Rows 0-20:  Play area (21 rows)
Row 21:     Grass (▓)
Rows 22-23: Dirt (░)
```

The bird starts at `y = height / 2.5 = 9.6` with an initial upward velocity of `flapStrength`. This places it in the upper third of the play area, rising — giving the player a moment to orient before gravity takes over.

**Why `height / 2.5` and not `height / 2`:** Starting dead center with an initial flap sends the bird toward the ceiling too quickly. Starting slightly lower gives more upward room for the initial arc.

## Relationship Between Constants

These constants are interdependent. Changing one usually requires adjusting others:

```
gravity ↔ flapStrength    Higher gravity needs stronger flaps
pipeGap ↔ gravity         Faster falling needs wider gaps
pipeSpeed ↔ pipeSpacing   Faster scroll needs wider spacing
pipeSpeed ↔ renderRate    Sub-pixel speed needs interpolation at high FPS
maxFallSpeed ↔ flapStrength  Cap should be ~2x |flapStrength| for reliable recovery
```

**The tuning loop:** Change a constant → play 5 games → does it feel right? → repeat. There is no formula. The numbers are the result of iterative play-testing.
