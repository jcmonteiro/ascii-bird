package main

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

// ──────────────────────────────────────────────
// Terminal helpers (raw mode, size, cursor, etc.)
// ──────────────────────────────────────────────

var origTermios *unix.Termios

func enableRawMode() {
	fd := int(os.Stdin.Fd())
	t, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		panic(err)
	}
	origTermios = t

	raw := *t
	raw.Iflag &^= unix.BRKINT | unix.ICRNL | unix.INPCK | unix.ISTRIP | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Cflag |= unix.CS8
	raw.Lflag &^= unix.ECHO | unix.ICANON | unix.IEXTEN | unix.ISIG
	raw.Cc[unix.VMIN] = 0
	raw.Cc[unix.VTIME] = 0

	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, &raw); err != nil {
		panic(err)
	}
}

func disableRawMode() {
	if origTermios != nil {
		unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TIOCSETA, origTermios)
	}
}

func getTermSize() (int, int) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 80, 24
	}
	return int(ws.Col), int(ws.Row)
}

func hideCursor()       { fmt.Print("\033[?25l") }
func showCursor()       { fmt.Print("\033[?25h") }
func clearScreen()      { fmt.Print("\033[2J") }
func moveCursor(r, c int) { fmt.Printf("\033[%d;%dH", r, c) }

// ──────────────────────────
// ANSI color helpers
// ──────────────────────────

func fg256(code int) string { return fmt.Sprintf("\033[38;5;%dm", code) }
func bg256(code int) string { return fmt.Sprintf("\033[48;5;%dm", code) }

const reset = "\033[0m"

// color palette
var (
	colSky       = bg256(117)                  // light blue bg
	colGround    = bg256(94) + fg256(94)       // brown
	colGrass     = bg256(34) + fg256(34)       // green grass top
	colPipe      = bg256(34) + fg256(22)       // green pipe
	colPipeCap   = bg256(28) + fg256(22)       // darker green cap
	colBirdBody  = fg256(226)                  // yellow
	colBirdWing  = fg256(214)                  // orange
	colBirdEye   = fg256(15)                   // white
	colBirdBeak  = fg256(208)                  // orange-red
	colScore     = fg256(15)                   // white text
	colTitle     = fg256(226)                  // yellow
	colSubtitle  = fg256(15)                   // white
	colGameOver  = fg256(196)                  // red
	colMedal     = fg256(220)                  // gold-ish
)

// ──────────────────────────
// Game constants
// ──────────────────────────

const (
	gravity      = 0.18
	flapStrength = -1.35
	maxFallSpeed = 2.8  // terminal velocity — prevents punishing freefall
	pipeWidth    = 6
	pipeGap      = 11   // vertical gap between top and bottom pipe
	pipeSpacing  = 25   // horizontal distance between pipes
	pipeSpeed    = 0.67 // pixels per physics tick (keeps ~20 px/sec at 30 ticks/sec)
	groundHeight = 3
	physicsRate  = 33 * time.Millisecond // 30 Hz — fixed simulation step
	renderRate   = 16 * time.Millisecond // ~60 FPS — visual refresh rate
)

// ──────────────────────────
// Data types
// ──────────────────────────

type Bird struct {
	y    float64
	vy   float64
	x    int // fixed column position on screen
	frame int // animation frame
}

type Pipe struct {
	x      int // current screen x position
	gapTop int // row where gap starts (top of opening)
	scored bool
}

type GameState int

const (
	StateTitle GameState = iota
	StatePlaying
	StateDead
)

type Game struct {
	width, height int
	bird          Bird
	pipes         []Pipe
	score         int
	bestScore     int
	state         GameState
	frameCount    int
	scrollOffset  float64     // smooth sub-pixel scroll offset (accumulated pipeSpeed)
	renderAlpha   float64     // interpolation fraction for render between physics steps
	renderBuf     bytes.Buffer // reusable render output buffer (retains allocation across frames)
	buf           [][]rune    // character buffer
	colBuf        [][]string  // color buffer (per cell)
}

// ──────────────────────────
// Game lifecycle
// ──────────────────────────

func NewGame() *Game {
	w, h := getTermSize()
	// Clamp to reasonable bounds
	if w > 120 { w = 120 }
	if h > 40 { h = 40 }

	return newGameWithSize(w, h)
}

// newGameWithSize creates a game with explicit dimensions (used by tests).
func newGameWithSize(w, h int) *Game {
	g := &Game{
		width:  w,
		height: h,
		state:  StateTitle,
	}
	g.resetBird()
	g.pipes = nil
	g.allocBuffers()
	return g
}

func (g *Game) allocBuffers() {
	g.buf = make([][]rune, g.height)
	g.colBuf = make([][]string, g.height)
	for r := range g.buf {
		g.buf[r] = make([]rune, g.width)
		g.colBuf[r] = make([]string, g.width)
	}
}

func (g *Game) resetBird() {
	g.bird = Bird{
		y:  float64(g.height) / 2.5,
		vy: 0,
		x:  g.width / 5,
	}
}

func (g *Game) startGame() {
	g.score = 0
	g.frameCount = 0
	g.scrollOffset = 0
	g.pipes = nil
	g.resetBird()
	g.bird.vy = flapStrength // start with an initial jump
	g.state = StatePlaying
	g.spawnInitialPipes()
}

func (g *Game) playArea() int {
	return g.height - groundHeight
}

func (g *Game) spawnInitialPipes() {
	startX := g.width + 10
	// Spawn enough pipes to always have one off-screen on each side.
	// This prevents visible pop-in on the right and pop-out on the left.
	numPipes := (g.width / pipeSpacing) + 3
	for i := 0; i < numPipes; i++ {
		g.pipes = append(g.pipes, g.makePipe(startX+i*pipeSpacing))
	}
}

func (g *Game) makePipe(x int) Pipe {
	playH := g.playArea()
	// gap can be between row 3 and playH - pipeGap - 3
	minGap := 3
	maxGap := playH - pipeGap - 3
	if maxGap < minGap+1 {
		maxGap = minGap + 1
	}
	gapTop := minGap + rand.Intn(maxGap-minGap)
	return Pipe{x: x, gapTop: gapTop}
}

// pipeScreenX returns the screen column for a pipe, applying the smooth scroll
// offset plus render interpolation for jitter-free sub-pixel positioning.
func (g *Game) pipeScreenX(p Pipe) int {
	smoothOffset := g.scrollOffset + pipeSpeed*g.renderAlpha
	return p.x - int(math.Round(smoothOffset))
}

// ──────────────────────────
// Input
// ──────────────────────────

// InputEvent represents a parsed input action
type InputEvent int

const (
	InputNone InputEvent = iota
	InputFlap
	InputQuit
)

func readInput(ch chan InputEvent, quit chan struct{}) {
	buf := make([]byte, 1)
	for {
		select {
		case <-quit:
			return
		default:
		}
		n, _ := os.Stdin.Read(buf[:1])
		if n == 0 {
			time.Sleep(1 * time.Millisecond) // yield CPU; prevents busy-wait with VMIN=0
			continue
		}

		switch buf[0] {
		case ' ', 'w', 'W':
			ch <- InputFlap
		case 'q', 'Q':
			ch <- InputQuit
		case 27: // ESC - could be ESC key alone or start of escape sequence
			// Wait briefly to see if more bytes follow (escape sequence)
			time.Sleep(25 * time.Millisecond)
			// Try to read the next byte (non-blocking since VMIN=0, VTIME=0)
			n2, _ := os.Stdin.Read(buf[:1])
			if n2 == 0 {
				// Standalone ESC key
				ch <- InputQuit
				continue
			}
			if buf[0] == '[' {
				// CSI sequence, read the final byte
				n3, _ := os.Stdin.Read(buf[:1])
				if n3 > 0 && buf[0] == 'A' {
					ch <- InputFlap // Up arrow
				}
				// Other arrow keys and sequences are ignored
			}
		}
	}
}

// ──────────────────────────
// Update
// ──────────────────────────

func (g *Game) update(flap bool) {
	if g.state != StatePlaying {
		return
	}

	g.frameCount++

	// Bird physics
	g.bird.vy += gravity
	if flap {
		g.bird.vy = flapStrength
	}
	// Terminal velocity cap — prevents punishing freefall
	if g.bird.vy > maxFallSpeed {
		g.bird.vy = maxFallSpeed
	}
	g.bird.y += g.bird.vy
	g.bird.frame = g.frameCount

	// Scroll: advance the smooth offset (pipes stay at base positions)
	g.scrollOffset += pipeSpeed

	// Score: bird passed a pipe
	for i := range g.pipes {
		sx := g.pipeScreenX(g.pipes[i])
		if !g.pipes[i].scored && sx+pipeWidth < g.bird.x {
			g.pipes[i].scored = true
			g.score++
		}
	}

	// Remove off-screen pipes, spawn new ones
	if len(g.pipes) > 0 && g.pipeScreenX(g.pipes[0]) < -(pipeWidth + 2) {
		g.pipes = g.pipes[1:]
		// spawn a new one beyond the right edge
		lastX := g.pipes[len(g.pipes)-1].x
		newX := lastX + pipeSpacing
		g.pipes = append(g.pipes, g.makePipe(newX))
	}

	// Collision detection
	if g.checkCollision() {
		g.die()
	}
}

func (g *Game) checkCollision() bool {
	birdRow := int(g.bird.y)
	birdCol := g.bird.x
	playH := g.playArea()

	// Ground or ceiling
	if birdRow >= playH-1 || birdRow <= 0 {
		return true
	}

	// Bird occupies roughly a 3-wide, 2-tall area
	birdLeft := birdCol - 1
	birdRight := birdCol + 1
	birdTop := birdRow
	birdBottom := birdRow + 1

	for _, p := range g.pipes {
		sx := g.pipeScreenX(p)
		pipeLeft := sx
		pipeRight := sx + pipeWidth - 1

		// Check horizontal overlap
		if birdRight >= pipeLeft && birdLeft <= pipeRight {
			// Check if bird is in the gap
			gapBottom := p.gapTop + pipeGap
			if birdTop < p.gapTop || birdBottom >= gapBottom {
				return true
			}
		}
	}

	return false
}

func (g *Game) die() {
	g.state = StateDead
	if g.score > g.bestScore {
		g.bestScore = g.score
	}
}

// ──────────────────────────
// Rendering
// ──────────────────────────

func (g *Game) clearBuf() {
	for r := 0; r < g.height; r++ {
		for c := 0; c < g.width; c++ {
			g.buf[r][c] = ' '
			g.colBuf[r][c] = colSky
		}
	}
}

func (g *Game) renderGround() {
	playH := g.playArea()
	// Grass line
	if playH < g.height {
		for c := 0; c < g.width; c++ {
			g.buf[playH][c] = '▓'
			g.colBuf[playH][c] = colGrass
		}
	}
	// Dirt
	for r := playH + 1; r < g.height; r++ {
		for c := 0; c < g.width; c++ {
			g.buf[r][c] = '░'
			g.colBuf[r][c] = colGround
		}
	}
}

func (g *Game) renderPipes() {
	playH := g.playArea()

	for _, p := range g.pipes {
		sx := g.pipeScreenX(p)
		gapBottom := p.gapTop + pipeGap

		for col := sx; col < sx+pipeWidth; col++ {
			if col < 0 || col >= g.width {
				continue
			}

			// Top pipe body
			for row := 0; row < p.gapTop; row++ {
				if row < 0 || row >= playH {
					continue
				}
				isEdge := col == sx || col == sx+pipeWidth-1
				if isEdge {
					g.buf[row][col] = '║'
				} else {
					g.buf[row][col] = '█'
				}
				g.colBuf[row][col] = colPipe
			}

			// Top pipe cap (the row just above the gap)
			capRow := p.gapTop - 1
			if capRow >= 0 && capRow < playH {
				// Widen cap by 1 on each side
				g.buf[capRow][col] = '▄'
				g.colBuf[capRow][col] = colPipeCap
			}
			// Extra cap width
			if col == sx && sx-1 >= 0 && p.gapTop-1 >= 0 && p.gapTop-1 < playH {
				g.buf[p.gapTop-1][sx-1] = '▄'
				g.colBuf[p.gapTop-1][sx-1] = colPipeCap
			}
			if col == sx+pipeWidth-1 && sx+pipeWidth < g.width && p.gapTop-1 >= 0 && p.gapTop-1 < playH {
				g.buf[p.gapTop-1][sx+pipeWidth] = '▄'
				g.colBuf[p.gapTop-1][sx+pipeWidth] = colPipeCap
			}

			// Bottom pipe cap (the row at gapBottom)
			capRowB := gapBottom
			if capRowB >= 0 && capRowB < playH {
				g.buf[capRowB][col] = '▀'
				g.colBuf[capRowB][col] = colPipeCap
			}
			if col == sx && sx-1 >= 0 && gapBottom >= 0 && gapBottom < playH {
				g.buf[gapBottom][sx-1] = '▀'
				g.colBuf[gapBottom][sx-1] = colPipeCap
			}
			if col == sx+pipeWidth-1 && sx+pipeWidth < g.width && gapBottom >= 0 && gapBottom < playH {
				g.buf[gapBottom][sx+pipeWidth] = '▀'
				g.colBuf[gapBottom][sx+pipeWidth] = colPipeCap
			}

			// Bottom pipe body
			for row := gapBottom + 1; row < playH; row++ {
				if row < 0 || row >= playH {
					continue
				}
				isEdge := col == sx || col == sx+pipeWidth-1
				if isEdge {
					g.buf[row][col] = '║'
				} else {
					g.buf[row][col] = '█'
				}
				g.colBuf[row][col] = colPipe
			}
		}
	}
}

func (g *Game) renderBird() {
	// Visual row uses rounding for smoother animation at arc apex.
	// Collision uses raw int(bird.y) separately — they are decoupled.
	row := int(math.Round(g.bird.y))
	col := g.bird.x
	playH := g.playArea()

	if row < 0 || row >= playH {
		return
	}

	// Bird is a 3-wide, 2-tall ASCII art character
	// Animate wing flapping
	wingUp := (g.bird.frame/4)%2 == 0

	// Row 0 of bird (top)
	//  ◔)    eye + beak
	if col-1 >= 0 && col-1 < g.width && row >= 0 && row < playH {
		g.buf[row][col-1] = '('
		g.colBuf[row][col-1] = colSky + colBirdBody
	}
	if col >= 0 && col < g.width && row >= 0 && row < playH {
		g.buf[row][col] = '◔'
		g.colBuf[row][col] = colSky + colBirdEye
	}
	if col+1 >= 0 && col+1 < g.width && row >= 0 && row < playH {
		g.buf[row][col+1] = '>'
		g.colBuf[row][col+1] = colSky + colBirdBeak
	}

	// Row 1 of bird (bottom) - body + wing
	if row+1 >= 0 && row+1 < playH {
		if col-1 >= 0 && col-1 < g.width {
			if wingUp {
				g.buf[row+1][col-1] = '~'
			} else {
				g.buf[row+1][col-1] = '='
			}
			g.colBuf[row+1][col-1] = colSky + colBirdWing
		}
		if col >= 0 && col < g.width {
			g.buf[row+1][col] = 'O'
			g.colBuf[row+1][col] = colSky + colBirdBody
		}
		if col+1 >= 0 && col+1 < g.width {
			g.buf[row+1][col+1] = '>'
			g.colBuf[row+1][col+1] = colSky + colBirdBeak
		}
	}
}

func (g *Game) renderScore() {
	scoreStr := fmt.Sprintf(" Score: %d ", g.score)
	row := 1
	col := (g.width - len(scoreStr)) / 2
	for i, ch := range scoreStr {
		c := col + i
		if c >= 0 && c < g.width && row >= 0 && row < g.height {
			g.buf[row][c] = ch
			g.colBuf[row][c] = colSky + "\033[1m" + colScore
		}
	}
}

func (g *Game) renderTitleScreen() {
	g.clearBuf()
	g.renderGround()

	// Animated bird on title screen
	titleBirdRow := g.height/2 - 2
	bobOffset := 0
	if (g.frameCount/8)%2 == 0 {
		bobOffset = -1
	}
	titleBirdRow += bobOffset

	col := g.width / 2

	// Bird
	if titleBirdRow >= 0 && titleBirdRow < g.playArea() {
		if col-1 >= 0 && col-1 < g.width {
			g.buf[titleBirdRow][col-1] = '('
			g.colBuf[titleBirdRow][col-1] = colSky + colBirdBody
		}
		if col < g.width {
			g.buf[titleBirdRow][col] = '◔'
			g.colBuf[titleBirdRow][col] = colSky + colBirdEye
		}
		if col+1 < g.width {
			g.buf[titleBirdRow][col+1] = '>'
			g.colBuf[titleBirdRow][col+1] = colSky + colBirdBeak
		}
	}
	if titleBirdRow+1 >= 0 && titleBirdRow+1 < g.playArea() {
		wingCh := '~'
		if bobOffset == 0 {
			wingCh = '='
		}
		if col-1 >= 0 && col-1 < g.width {
			g.buf[titleBirdRow+1][col-1] = wingCh
			g.colBuf[titleBirdRow+1][col-1] = colSky + colBirdWing
		}
		if col < g.width {
			g.buf[titleBirdRow+1][col] = 'O'
			g.colBuf[titleBirdRow+1][col] = colSky + colBirdBody
		}
		if col+1 < g.width {
			g.buf[titleBirdRow+1][col+1] = '>'
			g.colBuf[titleBirdRow+1][col+1] = colSky + colBirdBeak
		}
	}

	// Title text
	title := "ASCII BIRD"
	g.drawCentered(g.height/2-6, title, colSky+"\033[1m"+colTitle)

	subtitle := "A Flappy Bird Clone"
	g.drawCentered(g.height/2-4, subtitle, colSky+colSubtitle)

	// Instructions
	g.drawCentered(g.height/2+3, "Press SPACE or UP to flap", colSky+colSubtitle)
	g.drawCentered(g.height/2+5, "Press Q or ESC to quit", colSky+fg256(245))

	if g.bestScore > 0 {
		best := fmt.Sprintf("Best: %d", g.bestScore)
		g.drawCentered(g.height/2+7, best, colSky+"\033[1m"+colMedal)
	}
}

func (g *Game) renderGameOverOverlay() {
	// Semi-transparent overlay effect using darker background
	centerR := g.height / 2
	boxW := 30
	boxH := 11
	startC := (g.width - boxW) / 2
	startR := centerR - boxH/2

	// Draw box background
	for r := startR; r < startR+boxH; r++ {
		for c := startC; c < startC+boxW; c++ {
			if r >= 0 && r < g.height && c >= 0 && c < g.width {
				g.buf[r][c] = ' '
				g.colBuf[r][c] = bg256(236)
			}
		}
	}

	// Box border
	for c := startC; c < startC+boxW; c++ {
		if startR >= 0 && startR < g.height && c >= 0 && c < g.width {
			g.buf[startR][c] = '─'
			g.colBuf[startR][c] = bg256(236) + fg256(196)
		}
		br := startR + boxH - 1
		if br >= 0 && br < g.height && c >= 0 && c < g.width {
			g.buf[br][c] = '─'
			g.colBuf[br][c] = bg256(236) + fg256(196)
		}
	}
	for r := startR; r < startR+boxH; r++ {
		if r >= 0 && r < g.height {
			if startC >= 0 && startC < g.width {
				g.buf[r][startC] = '│'
				g.colBuf[r][startC] = bg256(236) + fg256(196)
			}
			ec := startC + boxW - 1
			if ec >= 0 && ec < g.width {
				g.buf[r][ec] = '│'
				g.colBuf[r][ec] = bg256(236) + fg256(196)
			}
		}
	}
	// Corners
	g.setCell(startR, startC, '┌', bg256(236)+fg256(196))
	g.setCell(startR, startC+boxW-1, '┐', bg256(236)+fg256(196))
	g.setCell(startR+boxH-1, startC, '└', bg256(236)+fg256(196))
	g.setCell(startR+boxH-1, startC+boxW-1, '┘', bg256(236)+fg256(196))

	// Content
	g.drawCenteredInBox(startR+2, startC, boxW, "GAME OVER", bg256(236)+"\033[1m"+colGameOver)

	scoreStr := fmt.Sprintf("Score: %d", g.score)
	g.drawCenteredInBox(startR+4, startC, boxW, scoreStr, bg256(236)+"\033[1m"+colScore)

	bestStr := fmt.Sprintf("Best:  %d", g.bestScore)
	g.drawCenteredInBox(startR+5, startC, boxW, bestStr, bg256(236)+colMedal)

	// Medal
	medal := g.getMedal()
	if medal != "" {
		g.drawCenteredInBox(startR+7, startC, boxW, medal, bg256(236)+"\033[1m"+colMedal)
	}

	g.drawCenteredInBox(startR+boxH-2, startC, boxW, "SPACE=Retry  Q=Quit", bg256(236)+fg256(245))
}

func (g *Game) getMedal() string {
	switch {
	case g.score >= 40:
		return "* PLATINUM *"
	case g.score >= 30:
		return "* GOLD *"
	case g.score >= 20:
		return "* SILVER *"
	case g.score >= 10:
		return "* BRONZE *"
	default:
		return ""
	}
}

func farewellMessage(best int) string {
	pick := func(opts []string) string {
		return opts[rand.Intn(len(opts))]
	}

	var quip string
	switch {
	case best == 0:
		quip = pick([]string{
			"You didn't even play. The bird died waiting for you.",
			"Zero. Not even one flap. The spacebar is right there.",
			"Did you open this by accident? Be honest.",
		})
	case best == 1:
		quip = pick([]string{
			"A score of 1. You technically played. Technically.",
			"One pipe. One! The tutorial would have been longer.",
			"Score: 1. That's not a high score, that's a rounding error.",
		})
	case best < 5:
		quip = pick([]string{
			"The pipes aren't even that close together. Just saying.",
			"Single digits. The bird believed in you. The bird was wrong.",
			"You'll get 'em next time. Probably. Maybe. No promises.",
			"Have you considered a game with fewer obstacles? Like Solitaire?",
		})
	case best < 10:
		quip = pick([]string{
			"Almost mediocre. That's a compliment in this game.",
			"Not bad! Well, not good either. But not bad.",
			"You can see double digits from here. Squint a little.",
			"Halfway to Bronze. The participation trophy is in the mail.",
		})
	case best < 20:
		quip = pick([]string{
			"Bronze tier! Your reflexes have a pulse after all.",
			"Double digits! Someone's been eating their vegetables.",
			"Bronze! That's third place if there were only three places.",
			"Hey, not terrible! We mean that in the nicest way possible.",
		})
	case best < 30:
		quip = pick([]string{
			"Silver! You're either getting good or getting lucky. We'll never know.",
			"Silver tier. The bird is cautiously impressed.",
			"Okay, you clearly have some idea what you're doing. Suspicious.",
			"Silver! Your hand-eye coordination is above room temperature.",
		})
	case best < 40:
		quip = pick([]string{
			"Gold! Genuinely impressive. Your parents would be proud, if they understood what a terminal is.",
			"Gold tier! You've spent more time on this than most people spend on hobbies.",
			"Gold. We'd clap but you can't hear us through the terminal.",
			"Seriously impressive. Wasted talent, but impressive.",
		})
	case best < 60:
		quip = pick([]string{
			"Platinum. Okay, you're actually good at this. Please go use these reflexes for something that pays.",
			"Platinum! At this point you're not playing the game, the game is playing you.",
			"Platinum tier. Your keyboard is scared of you and honestly, so are we.",
			"That score should be on a resume. Under 'questionable priorities.'",
		})
	case best < 80:
		quip = pick([]string{
			"Wow. That's... actually kind of beautiful. You've peaked, and it was in a terminal Flappy Bird clone.",
			"We didn't think scores like this were possible. We wrote the game and we can't do this.",
			"At this level, you're not a player. You're a force of nature with a spacebar.",
			"Magnificent. Terrifying. We're not sure which.",
		})
	case best < 100:
		quip = pick([]string{
			"Absolutely unhinged score. The bird respects you. We're a little scared of you.",
			"This score is a cry for help wrapped in extraordinary talent.",
			"You've transcended Flappy Bird. This is performance art now.",
			"We'd accuse you of cheating, but the code is right there and we'd know.",
		})
	default:
		quip = pick([]string{
			"Triple digits. You are a god among mortals and this ASCII bird is not worthy. Seek help.",
			"Over 100. In a terminal. With ASCII art. You absolute legend. Go outside.",
			"This score is so high it's basically a personality trait at this point.",
			"We bow before you. Now please close the terminal and touch grass.",
		})
	}
	return fmt.Sprintf("\n  ASCII Bird - Best: %d\n  %s\n", best, quip)
}

func (g *Game) setCell(r, c int, ch rune, col string) {
	if r >= 0 && r < g.height && c >= 0 && c < g.width {
		g.buf[r][c] = ch
		g.colBuf[r][c] = col
	}
}

func (g *Game) drawCentered(row int, text string, col string) {
	textLen := utf8.RuneCountInString(text)
	startC := (g.width - textLen) / 2
	for i, ch := range text {
		c := startC + i
		if c >= 0 && c < g.width && row >= 0 && row < g.height {
			g.buf[row][c] = ch
			g.colBuf[row][c] = col
		}
	}
}

func (g *Game) drawCenteredInBox(row, boxStart, boxW int, text string, col string) {
	textLen := utf8.RuneCountInString(text)
	startC := boxStart + (boxW-textLen)/2
	for i, ch := range text {
		c := startC + i
		if c >= 0 && c < g.width && row >= 0 && row < g.height {
			g.buf[row][c] = ch
			g.colBuf[row][c] = col
		}
	}
}

func (g *Game) render() string {
	g.renderBuf.Reset()

	for r := 0; r < g.height; r++ {
		fmt.Fprintf(&g.renderBuf, "\033[%d;%dH", r+1, 1)
		prevCol := ""
		for c := 0; c < g.width; c++ {
			col := g.colBuf[r][c]
			if col != prevCol {
				g.renderBuf.WriteString(col)
				prevCol = col
			}
			g.renderBuf.WriteRune(g.buf[r][c])
		}
		g.renderBuf.WriteString(reset)
	}
	return g.renderBuf.String()
}

// bufText extracts plain text from the character buffer (no ANSI codes).
// Each row becomes a line in the returned string.
func (g *Game) bufText() string {
	var sb strings.Builder
	for r := 0; r < g.height; r++ {
		for c := 0; c < g.width; c++ {
			sb.WriteRune(g.buf[r][c])
		}
		if r < g.height-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// bufRow returns the plain text of a single row from the character buffer.
func (g *Game) bufRow(row int) string {
	if row < 0 || row >= g.height {
		return ""
	}
	var sb strings.Builder
	for c := 0; c < g.width; c++ {
		sb.WriteRune(g.buf[row][c])
	}
	return sb.String()
}

// ──────────────────────────
// Clouds (background decoration)
// ──────────────────────────

type Cloud struct {
	row, col int
	style    int
}

var clouds []Cloud

func initClouds(w, h int) {
	clouds = nil
	playH := h - groundHeight
	for i := 0; i < 5; i++ {
		clouds = append(clouds, Cloud{
			row:   2 + rand.Intn(playH/2),
			col:   rand.Intn(w),
			style: rand.Intn(3),
		})
	}
}

func (g *Game) renderClouds() {
	cloudChars := []string{
		"  ._===_.  ",
		"   .-=-.   ",
		" .--===--. ",
	}

	for _, cl := range clouds {
		art := cloudChars[cl.style%len(cloudChars)]
		for i, ch := range art {
			c := cl.col + i
			r := cl.row
			if c >= 0 && c < g.width && r >= 0 && r < g.playArea() {
				if g.buf[r][c] == ' ' { // don't draw over pipes/bird
					g.buf[r][c] = ch
					g.colBuf[r][c] = colSky + fg256(153) // light cloud color
				}
			}
		}
	}
}

func scrollClouds(w int) {
	for i := range clouds {
		clouds[i].col--
		if clouds[i].col < -15 {
			clouds[i].col = w + rand.Intn(20)
			clouds[i].row = 2 + rand.Intn(8)
		}
	}
}

// ──────────────────────────
// Main game loop
// ──────────────────────────

func main() {
	rand.Seed(time.Now().UnixNano())

	enableRawMode()
	defer disableRawMode()
	hideCursor()
	defer showCursor()
	clearScreen()

	// Handle signals gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		showCursor()
		disableRawMode()
		clearScreen()
		moveCursor(1, 1)
		fmt.Println("Goodbye!")
		os.Exit(0)
	}()

	g := NewGame()
	initClouds(g.width, g.height)

	inputCh := make(chan InputEvent, 32)
	quitCh := make(chan struct{})
	var once sync.Once

	go readInput(inputCh, quitCh)

	// Render ticker drives the main loop at ~60 FPS.
	// Physics runs in fixed steps accumulated per render frame,
	// so game behavior is independent of render rate.
	renderTicker := time.NewTicker(renderRate)
	defer renderTicker.Stop()

	cloudTicker := time.NewTicker(150 * time.Millisecond)
	defer cloudTicker.Stop()

	lastPhysics := time.Now()
	physicsAccum := time.Duration(0)

	// Input state persists across render frames so inputs aren't
	// dropped when no physics step runs in a given render frame.
	pendingFlap := false
	pendingQuit := false

	for {
		select {
		case now := <-renderTicker.C:
			// Drain input — accumulate into persistent flags
		drainLoop:
			for {
				select {
				case ev := <-inputCh:
					switch ev {
					case InputFlap:
						pendingFlap = true
					case InputQuit:
						pendingQuit = true
					}
				default:
					break drainLoop
				}
			}

			if pendingQuit {
				pendingQuit = false
				if g.state == StatePlaying {
					g.die()
				} else {
				once.Do(func() { close(quitCh) })
				showCursor()
				disableRawMode()
				clearScreen()
				moveCursor(1, 1)
				fmt.Println(farewellMessage(g.bestScore))
				return
				}
			}

			// Accumulate elapsed time and run physics in fixed steps
			physicsAccum += now.Sub(lastPhysics)
			lastPhysics = now

			switch g.state {
			case StateTitle:
				for physicsAccum >= physicsRate {
					physicsAccum -= physicsRate
					g.frameCount++
					scrollClouds(g.width)
				}
				if pendingFlap {
					pendingFlap = false
					g.startGame()
				}
				g.renderTitleScreen()
				output := g.render()
				fmt.Print(output)

			case StatePlaying:
				// Run zero or more physics steps per render frame
				for physicsAccum >= physicsRate {
					physicsAccum -= physicsRate
					g.renderAlpha = 0 // physics uses exact offset
					g.update(pendingFlap)
					pendingFlap = false // consumed on first physics tick
				}
				// Interpolate for smooth rendering between physics ticks
				g.renderAlpha = float64(physicsAccum) / float64(physicsRate)
				g.clearBuf()
				g.renderGround()
				g.renderClouds()
				g.renderPipes()
				g.renderBird()
				g.renderScore()
				if g.state == StateDead {
					g.renderGameOverOverlay()
				}
				output := g.render()
				fmt.Print(output)

			case StateDead:
				if pendingFlap {
					pendingFlap = false
					g.startGame()
					initClouds(g.width, g.height)
				}
				g.renderAlpha = 0 // no interpolation when dead
				g.clearBuf()
				g.renderGround()
				g.renderClouds()
				g.renderPipes()
				g.renderBird()
				g.renderScore()
				g.renderGameOverOverlay()
				output := g.render()
				fmt.Print(output)
			}

		case <-cloudTicker.C:
			if g.state != StateTitle {
				scrollClouds(g.width)
			}
		}
	}
}
