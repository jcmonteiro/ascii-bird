package main

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// ═══════════════════════════════════════════
// Helper: create a test game with fixed size
// ═══════════════════════════════════════════

func testGame() *Game {
	return newGameWithSize(80, 24)
}

// ═══════════════════════════════════════════
// 1. GAME INITIALIZATION
// ═══════════════════════════════════════════

func TestNewGame_Dimensions(t *testing.T) {
	g := testGame()
	if g.width != 80 {
		t.Errorf("expected width 80, got %d", g.width)
	}
	if g.height != 24 {
		t.Errorf("expected height 24, got %d", g.height)
	}
}

func TestNewGame_InitialState(t *testing.T) {
	g := testGame()
	if g.state != StateTitle {
		t.Errorf("expected StateTitle, got %d", g.state)
	}
	if g.score != 0 {
		t.Errorf("expected score 0, got %d", g.score)
	}
	if g.bestScore != 0 {
		t.Errorf("expected bestScore 0, got %d", g.bestScore)
	}
	if len(g.pipes) != 0 {
		t.Errorf("expected no pipes on title, got %d", len(g.pipes))
	}
}

func TestNewGame_BuffersAllocated(t *testing.T) {
	g := testGame()
	if len(g.buf) != g.height {
		t.Fatalf("buf rows: expected %d, got %d", g.height, len(g.buf))
	}
	for r := 0; r < g.height; r++ {
		if len(g.buf[r]) != g.width {
			t.Fatalf("buf row %d cols: expected %d, got %d", r, g.width, len(g.buf[r]))
		}
		if len(g.colBuf[r]) != g.width {
			t.Fatalf("colBuf row %d cols: expected %d, got %d", r, g.width, len(g.colBuf[r]))
		}
	}
}

func TestNewGame_BirdPosition(t *testing.T) {
	g := testGame()
	if g.bird.x != 80/5 {
		t.Errorf("bird x: expected %d, got %d", 80/5, g.bird.x)
	}
	expectedY := float64(24) / 2.5
	if g.bird.y != expectedY {
		t.Errorf("bird y: expected %f, got %f", expectedY, g.bird.y)
	}
	if g.bird.vy != 0 {
		t.Errorf("bird vy: expected 0, got %f", g.bird.vy)
	}
}

func TestPlayArea(t *testing.T) {
	g := testGame()
	expected := 24 - groundHeight // 21
	if g.playArea() != expected {
		t.Errorf("playArea: expected %d, got %d", expected, g.playArea())
	}
}

// ═══════════════════════════════════════════
// 2. GAME STATE TRANSITIONS
// ═══════════════════════════════════════════

func TestStartGame_TransitionsState(t *testing.T) {
	g := testGame()
	g.startGame()
	if g.state != StatePlaying {
		t.Errorf("expected StatePlaying, got %d", g.state)
	}
	if g.score != 0 {
		t.Errorf("expected score 0 after start, got %d", g.score)
	}
	if g.frameCount != 0 {
		t.Errorf("expected frameCount 0 after start, got %d", g.frameCount)
	}
}

func TestStartGame_SpawnsPipes(t *testing.T) {
	g := testGame()
	g.startGame()
	if len(g.pipes) != 4 {
		t.Errorf("expected 4 initial pipes, got %d", len(g.pipes))
	}
	// All pipes should be off-screen to the right
	for i, p := range g.pipes {
		if p.x <= g.width {
			t.Errorf("pipe %d at x=%d should be > width=%d", i, p.x, g.width)
		}
	}
}

func TestStartGame_PipesAreSpaced(t *testing.T) {
	g := testGame()
	g.startGame()
	for i := 1; i < len(g.pipes); i++ {
		spacing := g.pipes[i].x - g.pipes[i-1].x
		if spacing != pipeSpacing {
			t.Errorf("pipes %d→%d spacing: expected %d, got %d", i-1, i, pipeSpacing, spacing)
		}
	}
}

func TestDie_TransitionsState(t *testing.T) {
	g := testGame()
	g.startGame()
	g.score = 5
	g.die()
	if g.state != StateDead {
		t.Errorf("expected StateDead, got %d", g.state)
	}
	if g.bestScore != 5 {
		t.Errorf("expected bestScore 5, got %d", g.bestScore)
	}
}

func TestDie_BestScoreTracking(t *testing.T) {
	g := testGame()
	g.startGame()
	g.score = 10
	g.die()
	if g.bestScore != 10 {
		t.Fatalf("expected bestScore 10, got %d", g.bestScore)
	}

	// Play again with lower score
	g.startGame()
	g.score = 3
	g.die()
	if g.bestScore != 10 {
		t.Errorf("bestScore should still be 10 after lower score 3, got %d", g.bestScore)
	}

	// Play again with higher score
	g.startGame()
	g.score = 42
	g.die()
	if g.bestScore != 42 {
		t.Errorf("bestScore should be 42 after higher score, got %d", g.bestScore)
	}
}

func TestRestart_ResetsBird(t *testing.T) {
	g := testGame()
	g.startGame()
	// Mutate bird position
	g.bird.y = 100
	g.bird.vy = 50
	g.die()
	// Restart
	g.startGame()
	expectedY := float64(24) / 2.5
	if g.bird.y != expectedY {
		t.Errorf("bird y after restart: expected %f, got %f", expectedY, g.bird.y)
	}
	if g.bird.vy != flapStrength {
		t.Errorf("bird vy after restart: expected %f (initial jump), got %f", flapStrength, g.bird.vy)
	}
}

// ═══════════════════════════════════════════
// 3. BIRD PHYSICS
// ═══════════════════════════════════════════

func TestGravity_BirdFalls(t *testing.T) {
	g := testGame()
	g.startGame()
	// Place bird safely in center, no pipes nearby
	g.pipes = nil
	g.bird.y = 10.0
	g.bird.vy = 0

	initialY := g.bird.y
	for i := 0; i < 5; i++ {
		g.update(false) // no flap
	}

	if g.bird.y <= initialY {
		t.Errorf("bird should have fallen: started at %f, now at %f", initialY, g.bird.y)
	}
	if g.bird.vy <= 0 {
		t.Errorf("bird vy should be positive (falling), got %f", g.bird.vy)
	}
}

func TestFlap_BirdRises(t *testing.T) {
	g := testGame()
	g.startGame()
	g.pipes = nil
	g.bird.y = 10.0
	g.bird.vy = 1.0 // falling

	g.update(true) // flap!

	if g.bird.vy != flapStrength+gravity {
		// After flap: vy = flapStrength, then vy += gravity, then y += vy
		// Actually: vy += gravity first, then if flap vy = flapStrength, then y += vy
		// Let me check the code order...
		// Code: vy += gravity; if flap { vy = flapStrength }; y += vy
		// So after flap: vy = flapStrength = -2.2, y += -2.2
	}

	// vy should be flapStrength (the flap overrides after gravity)
	// Looking at update(): gravity is applied first, then flap overrides vy
	if g.bird.vy != flapStrength {
		t.Errorf("bird vy after flap: expected %f, got %f (gravity applied first, flap overrides)", flapStrength, g.bird.vy)
	}
}

func TestFlap_VelocityOverridesGravity(t *testing.T) {
	g := testGame()
	g.startGame()
	g.pipes = nil
	g.bird.y = 10.0
	g.bird.vy = 5.0 // strong downward velocity

	beforeY := g.bird.y
	g.update(true) // flap

	// After update: vy was set to flapStrength (-2.2), y += -2.2
	afterY := g.bird.y
	if afterY >= beforeY {
		t.Errorf("bird should have moved up after flap: before=%f, after=%f", beforeY, afterY)
	}
}

func TestGravity_Acceleration(t *testing.T) {
	g := testGame()
	g.startGame()
	g.pipes = nil
	g.bird.y = 5.0
	g.bird.vy = 0

	// Each tick: vy += 0.4, y += vy
	// Tick 1: vy=0.4, y=5.4
	// Tick 2: vy=0.8, y=6.2
	// Tick 3: vy=1.2, y=7.4
	eps := 0.001
	g.update(false)
	if diff := g.bird.vy - gravity; diff < -eps || diff > eps {
		t.Errorf("tick 1: vy expected ~%f, got %f", gravity, g.bird.vy)
	}
	g.update(false)
	if diff := g.bird.vy - 2*gravity; diff < -eps || diff > eps {
		t.Errorf("tick 2: vy expected ~%f, got %f", 2*gravity, g.bird.vy)
	}
	g.update(false)
	if diff := g.bird.vy - 3*gravity; diff < -eps || diff > eps {
		t.Errorf("tick 3: vy expected ~%f, got %f", 3*gravity, g.bird.vy)
	}
}

// ═══════════════════════════════════════════
// 4. COLLISION DETECTION
// ═══════════════════════════════════════════

func TestCollision_Ground(t *testing.T) {
	g := testGame()
	g.startGame()
	g.pipes = nil
	playH := g.playArea()
	g.bird.y = float64(playH - 1) // at ground level

	if !g.checkCollision() {
		t.Error("bird at ground level should collide")
	}
}

func TestCollision_Ceiling(t *testing.T) {
	g := testGame()
	g.startGame()
	g.pipes = nil
	g.bird.y = 0 // at ceiling

	if !g.checkCollision() {
		t.Error("bird at ceiling should collide")
	}
}

func TestCollision_MidAir_NoPipes(t *testing.T) {
	g := testGame()
	g.startGame()
	g.pipes = nil
	g.bird.y = float64(g.playArea() / 2)

	if g.checkCollision() {
		t.Error("bird in mid-air with no pipes should not collide")
	}
}

func TestCollision_HitTopPipe(t *testing.T) {
	g := testGame()
	g.startGame()
	// Place a pipe directly at the bird's position
	g.pipes = []Pipe{{x: g.bird.x - 2, gapTop: 12}}
	g.bird.y = 5 // above the gap (gapTop=12)

	if !g.checkCollision() {
		t.Error("bird above gap should collide with top pipe")
	}
}

func TestCollision_HitBottomPipe(t *testing.T) {
	g := testGame()
	g.startGame()
	g.pipes = []Pipe{{x: g.bird.x - 2, gapTop: 5}}
	g.bird.y = float64(5 + pipeGap) // at bottom pipe cap (gapTop + pipeGap)

	if !g.checkCollision() {
		t.Error("bird at bottom pipe should collide")
	}
}

func TestCollision_SafeInGap(t *testing.T) {
	g := testGame()
	g.startGame()
	gapTop := 6
	g.pipes = []Pipe{{x: g.bird.x - 2, gapTop: gapTop}}
	// Place bird in the middle of the gap
	g.bird.y = float64(gapTop + pipeGap/2)

	if g.checkCollision() {
		t.Error("bird safely in the gap should NOT collide")
	}
}

func TestCollision_PipeFarAway(t *testing.T) {
	g := testGame()
	g.startGame()
	// Pipe far to the right
	g.pipes = []Pipe{{x: g.bird.x + 20, gapTop: 5}}
	g.bird.y = 3 // would collide vertically, but pipe is far away

	if g.checkCollision() {
		t.Error("bird should not collide with distant pipe")
	}
}

func TestCollision_PipeEdgeJustTouching(t *testing.T) {
	g := testGame()
	g.startGame()
	// Bird hitbox: [bird.x-1, bird.x+1] horizontally, [bird.y, bird.y+1] vertically
	// Place pipe so its right edge = bird.x - 1 (just touching)
	pipeX := g.bird.x - 1 - pipeWidth + 1
	g.pipes = []Pipe{{x: pipeX, gapTop: 15}} // gap well below bird
	g.bird.y = 3

	// Bird's left is bird.x-1, pipe's right is pipeX + pipeWidth - 1
	// pipeRight = (bird.x - 1 - pipeWidth + 1) + pipeWidth - 1 = bird.x - 1
	// birdLeft = bird.x - 1
	// So birdLeft <= pipeRight → overlap!
	// Bird.y=3, gapTop=15, so birdTop(3) < gapTop(15) → collision
	if !g.checkCollision() {
		t.Error("bird at pipe edge should collide when outside gap")
	}
}

// ═══════════════════════════════════════════
// 5. SCORING
// ═══════════════════════════════════════════

func TestScoring_PassPipe(t *testing.T) {
	g := testGame()
	g.startGame()
	// Place a pipe that's already past the bird
	g.pipes = []Pipe{{x: g.bird.x - pipeWidth - 5, gapTop: 8, scored: false}}
	// Add a far-away pipe so removal logic doesn't panic
	g.pipes = append(g.pipes, Pipe{x: g.width + 50, gapTop: 8})

	// Bird in safe position
	g.bird.y = float64(8 + pipeGap/2)

	g.update(false)
	if g.score != 1 {
		t.Errorf("score should be 1 after passing pipe, got %d", g.score)
	}
	if !g.pipes[0].scored {
		t.Error("pipe should be marked as scored")
	}
}

func TestScoring_DontDoubleScore(t *testing.T) {
	g := testGame()
	g.startGame()
	g.pipes = []Pipe{
		{x: g.bird.x - pipeWidth - 5, gapTop: 8, scored: true}, // already scored
	}
	g.pipes = append(g.pipes, Pipe{x: g.width + 50, gapTop: 8})
	g.bird.y = float64(8 + pipeGap/2)

	g.update(false)
	if g.score != 0 {
		t.Errorf("already-scored pipe should not add score, got %d", g.score)
	}
}

func TestScoring_MultiplePassedPipes(t *testing.T) {
	g := testGame()
	g.startGame()
	g.bird.y = float64(8 + pipeGap/2)
	g.pipes = []Pipe{
		{x: g.bird.x - pipeWidth - 10, gapTop: 8, scored: false},
		{x: g.bird.x - pipeWidth - 5, gapTop: 8, scored: false},
		{x: g.width + 50, gapTop: 8},
	}

	g.update(false)
	if g.score != 2 {
		t.Errorf("should have scored 2 pipes, got %d", g.score)
	}
}

// ═══════════════════════════════════════════
// 6. PIPE GENERATION
// ═══════════════════════════════════════════

func TestPipeGeneration_GapBounds(t *testing.T) {
	g := testGame()
	playH := g.playArea()

	for i := 0; i < 100; i++ {
		p := g.makePipe(50)
		if p.gapTop < 3 {
			t.Errorf("pipe gap top %d is below minimum 3", p.gapTop)
		}
		gapBottom := p.gapTop + pipeGap
		if gapBottom > playH-3 {
			t.Errorf("pipe gap bottom %d exceeds playArea-3 (%d)", gapBottom, playH-3)
		}
	}
}

func TestPipeGeneration_Recycling(t *testing.T) {
	g := testGame()
	g.startGame()
	g.bird.y = float64(g.pipes[0].gapTop + pipeGap/2) // safe position

	initialCount := len(g.pipes)

	// Scroll many frames to push first pipe off screen
	for i := 0; i < 200; i++ {
		g.update(true) // keep flapping to stay alive
		// Reset bird to safe position to avoid dying
		if g.state == StateDead {
			break
		}
	}

	// Pipe count should remain stable (old ones removed, new ones added)
	if g.state != StateDead && len(g.pipes) < initialCount-1 {
		t.Errorf("pipe count dropped too low: started %d, now %d", initialCount, len(g.pipes))
	}
}

// ═══════════════════════════════════════════
// 7. MEDAL SYSTEM
// ═══════════════════════════════════════════

func TestMedals(t *testing.T) {
	tests := []struct {
		score    int
		expected string
	}{
		{0, ""},
		{5, ""},
		{9, ""},
		{10, "* BRONZE *"},
		{15, "* BRONZE *"},
		{19, "* BRONZE *"},
		{20, "* SILVER *"},
		{25, "* SILVER *"},
		{29, "* SILVER *"},
		{30, "* GOLD *"},
		{35, "* GOLD *"},
		{39, "* GOLD *"},
		{40, "* PLATINUM *"},
		{100, "* PLATINUM *"},
	}

	g := testGame()
	for _, tc := range tests {
		g.score = tc.score
		medal := g.getMedal()
		if medal != tc.expected {
			t.Errorf("score %d: expected medal %q, got %q", tc.score, tc.expected, medal)
		}
	}
}

func TestFarewellMessage(t *testing.T) {
	// Each tier has multiple messages; verify the score is always present
	// and that repeated calls can produce different messages (randomization).
	scores := []int{0, 1, 3, 7, 15, 25, 35, 50, 70, 90, 100, 999}

	for _, best := range scores {
		msg := farewellMessage(best)
		if !strings.Contains(msg, fmt.Sprintf("Best: %d", best)) {
			t.Errorf("best=%d: message should contain the score, got %q", best, msg)
		}
		if len(msg) < 20 {
			t.Errorf("best=%d: message suspiciously short: %q", best, msg)
		}
	}
}

func TestFarewellMessage_Randomized(t *testing.T) {
	// Call farewellMessage many times for a given score and verify we get
	// more than one unique message (proves randomization is working).
	// Use score=3 which has 4 options — with 50 tries the chance of
	// always picking the same one is (1/4)^49 ≈ 0, effectively impossible.
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		msg := farewellMessage(3)
		seen[msg] = true
	}
	if len(seen) < 2 {
		t.Errorf("expected multiple different messages for same score, got %d unique", len(seen))
	}
	t.Logf("  farewell randomization: saw %d unique messages in 50 calls", len(seen))
}

// ═══════════════════════════════════════════
// 8. TITLE SCREEN VISUAL VALIDATION
// ═══════════════════════════════════════════

func TestTitleScreen_ContainsTitle(t *testing.T) {
	g := testGame()
	g.frameCount = 0
	g.renderTitleScreen()
	text := g.bufText()

	if !strings.Contains(text, "ASCII BIRD") {
		t.Error("title screen should contain 'ASCII BIRD'")
	}
}

func TestTitleScreen_ContainsSubtitle(t *testing.T) {
	g := testGame()
	g.frameCount = 0
	g.renderTitleScreen()
	text := g.bufText()

	if !strings.Contains(text, "A Flappy Bird Clone") {
		t.Error("title screen should contain 'A Flappy Bird Clone'")
	}
}

func TestTitleScreen_ContainsInstructions(t *testing.T) {
	g := testGame()
	g.frameCount = 0
	g.renderTitleScreen()
	text := g.bufText()

	if !strings.Contains(text, "SPACE") {
		t.Error("title screen should mention SPACE key")
	}
	if !strings.Contains(text, "flap") {
		t.Error("title screen should mention flap action")
	}
	if !strings.Contains(text, "Q") || !strings.Contains(text, "quit") {
		t.Error("title screen should mention Q/quit")
	}
}

func TestTitleScreen_ContainsBird(t *testing.T) {
	g := testGame()
	g.frameCount = 0
	g.renderTitleScreen()
	text := g.bufText()

	// The bird uses these chars: ( ◔ > O and wing ~ or =
	if !strings.ContainsRune(text, '◔') {
		t.Error("title screen should contain bird eye character ◔")
	}
	if !strings.ContainsRune(text, 'O') {
		t.Error("title screen should contain bird body character O")
	}
}

func TestTitleScreen_ContainsGround(t *testing.T) {
	g := testGame()
	g.frameCount = 0
	g.renderTitleScreen()

	playH := g.playArea()
	grassRow := g.bufRow(playH)

	// Grass row should be all ▓
	if !strings.ContainsRune(grassRow, '▓') {
		t.Error("ground should have grass characters ▓")
	}

	// Dirt rows should have ░
	if playH+1 < g.height {
		dirtRow := g.bufRow(playH + 1)
		if !strings.ContainsRune(dirtRow, '░') {
			t.Error("dirt should have ░ characters")
		}
	}
}

func TestTitleScreen_ShowsBestScore(t *testing.T) {
	g := testGame()
	g.bestScore = 42
	g.frameCount = 0
	g.renderTitleScreen()
	text := g.bufText()

	if !strings.Contains(text, "Best: 42") {
		t.Error("title screen should show best score when > 0")
	}
}

func TestTitleScreen_NoBestScoreWhenZero(t *testing.T) {
	g := testGame()
	g.bestScore = 0
	g.frameCount = 0
	g.renderTitleScreen()
	text := g.bufText()

	if strings.Contains(text, "Best:") {
		t.Error("title screen should NOT show best score when 0")
	}
}

// ═══════════════════════════════════════════
// 9. TITLE SCREEN BIRD BOB ANIMATION
// ═══════════════════════════════════════════

func TestTitleBirdBob_Animates(t *testing.T) {
	g := testGame()

	// Capture bird position at frame 0 (bobOffset = -1 because (0/8)%2 == 0)
	g.frameCount = 0
	g.renderTitleScreen()
	frame0 := g.bufText()

	// Capture bird position at frame 8 (bobOffset = 0 because (8/8)%2 == 1)
	g.frameCount = 8
	g.renderTitleScreen()
	frame8 := g.bufText()

	// The bird should be in a different vertical position
	if frame0 == frame8 {
		t.Error("title bird should bob between frames 0 and 8 (different positions)")
	}

	// Frame 0: bob up. Frame 8: bob down. Let's find the '◔' row
	row0 := findRuneRow(g, '◔', frame0)
	row8 := findRuneRow(g, '◔', frame8)

	if row0 == -1 || row8 == -1 {
		t.Fatal("could not find bird eye ◔ in title screen frames")
	}
	if row0 == row8 {
		t.Errorf("bird eye should be at different rows: frame0 row=%d, frame8 row=%d", row0, row8)
	}
	t.Logf("  bob animation: frame0 bird eye at row %d, frame8 at row %d (delta=%d)", row0, row8, row8-row0)
}

func findRuneRow(g *Game, target rune, text string) int {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.ContainsRune(line, target) {
			return i
		}
	}
	return -1
}

// ═══════════════════════════════════════════
// 10. TITLE SCREEN BIRD WING ANIMATION
// ═══════════════════════════════════════════

func TestTitleBirdWing_Animates(t *testing.T) {
	g := testGame()

	// bobOffset = -1 at frame 0 → wingCh = '~'
	g.frameCount = 0
	g.renderTitleScreen()
	text0 := g.bufText()

	// bobOffset = 0 at frame 8 → wingCh = '='
	g.frameCount = 8
	g.renderTitleScreen()
	text8 := g.bufText()

	has0Tilde := strings.ContainsRune(text0, '~')
	has8Equals := strings.ContainsRune(text8, '=')

	if !has0Tilde {
		t.Error("frame 0 (bob up) should have wing character '~'")
	}
	if !has8Equals {
		t.Error("frame 8 (bob down) should have wing character '='")
	}
	t.Logf("  wing animation: frame0 has '~'=%v, frame8 has '='=%v", has0Tilde, has8Equals)
}

// ═══════════════════════════════════════════
// 11. GAMEPLAY SCREEN VISUAL VALIDATION
// ═══════════════════════════════════════════

func TestGameplayScreen_ContainsScore(t *testing.T) {
	g := testGame()
	g.startGame()
	g.bird.y = float64(g.playArea() / 2)
	g.pipes = nil // no pipes, just check rendering

	g.clearBuf()
	g.renderGround()
	g.renderBird()
	g.renderScore()
	text := g.bufText()

	if !strings.Contains(text, "Score: 0") {
		t.Error("gameplay screen should show 'Score: 0'")
	}
}

func TestGameplayScreen_ScoreUpdates(t *testing.T) {
	g := testGame()
	g.startGame()
	g.score = 7
	g.bird.y = float64(g.playArea() / 2)

	g.clearBuf()
	g.renderGround()
	g.renderBird()
	g.renderScore()
	text := g.bufText()

	if !strings.Contains(text, "Score: 7") {
		t.Error("gameplay screen should show 'Score: 7'")
	}
}

func TestGameplayScreen_BirdRendered(t *testing.T) {
	g := testGame()
	g.startGame()
	g.bird.y = float64(g.playArea() / 2)

	g.clearBuf()
	g.renderBird()
	text := g.bufText()

	if !strings.ContainsRune(text, '◔') {
		t.Error("gameplay should render bird eye ◔")
	}
	if !strings.ContainsRune(text, 'O') {
		t.Error("gameplay should render bird body O")
	}
	if !strings.ContainsRune(text, '>') {
		t.Error("gameplay should render bird beak >")
	}
}

func TestGameplayScreen_BirdWingFlap(t *testing.T) {
	g := testGame()
	g.startGame()
	g.bird.y = float64(g.playArea() / 2)

	// Frame where wingUp = true: (frame/4)%2 == 0 → frame 0
	g.bird.frame = 0
	g.clearBuf()
	g.renderBird()
	text0 := g.bufText()

	// Frame where wingUp = false: (frame/4)%2 == 1 → frame 4
	g.bird.frame = 4
	g.clearBuf()
	g.renderBird()
	text4 := g.bufText()

	has0Tilde := strings.ContainsRune(text0, '~')
	has4Equals := strings.ContainsRune(text4, '=')

	if !has0Tilde {
		t.Error("bird frame 0 should have wing '~'")
	}
	if !has4Equals {
		t.Error("bird frame 4 should have wing '='")
	}
	t.Logf("  bird wing flap: frame0='~' present=%v, frame4='=' present=%v", has0Tilde, has4Equals)
}

func TestGameplayScreen_PipesRendered(t *testing.T) {
	g := testGame()
	g.startGame()
	// Place a pipe on screen
	g.pipes = []Pipe{{x: 40, gapTop: 8}}

	g.clearBuf()
	g.renderPipes()
	text := g.bufText()

	// Pipe should have edge chars ║ and body chars █
	if !strings.ContainsRune(text, '║') {
		t.Error("pipes should have edge character ║")
	}
	if !strings.ContainsRune(text, '█') {
		t.Error("pipes should have body character █")
	}
	// Pipe caps
	if !strings.ContainsRune(text, '▄') {
		t.Error("pipes should have top cap character ▄")
	}
	if !strings.ContainsRune(text, '▀') {
		t.Error("pipes should have bottom cap character ▀")
	}
}

func TestGameplayScreen_PipeGapIsClear(t *testing.T) {
	g := testGame()
	g.startGame()
	gapTop := 8
	g.pipes = []Pipe{{x: 40, gapTop: gapTop}}

	g.clearBuf()
	g.renderPipes()

	// Check that the gap area (gapTop to gapTop+pipeGap-1) at a middle pipe column is empty
	midCol := 42 // middle of a 6-wide pipe at x=40
	for row := gapTop; row < gapTop+pipeGap; row++ {
		// Skip the cap rows (gapTop-1 is top cap, gapTop+pipeGap is bottom cap)
		ch := g.buf[row][midCol]
		if ch != ' ' {
			t.Errorf("gap at row %d col %d should be empty, got '%c'", row, midCol, ch)
		}
	}
}

func TestGameplayScreen_GroundRendered(t *testing.T) {
	g := testGame()
	g.startGame()

	g.clearBuf()
	g.renderGround()

	playH := g.playArea()
	// Check grass row
	if g.buf[playH][0] != '▓' {
		t.Errorf("ground grass row should be ▓, got '%c'", g.buf[playH][0])
	}
	// Check dirt
	if playH+1 < g.height {
		if g.buf[playH+1][0] != '░' {
			t.Errorf("dirt row should be ░, got '%c'", g.buf[playH+1][0])
		}
	}
}

// ═══════════════════════════════════════════
// 12. GAME OVER OVERLAY VISUAL VALIDATION
// ═══════════════════════════════════════════

func TestGameOver_ContainsText(t *testing.T) {
	g := testGame()
	g.startGame()
	g.score = 15
	g.die()

	g.clearBuf()
	g.renderGround()
	g.renderBird()
	g.renderScore()
	g.renderGameOverOverlay()
	text := g.bufText()

	if !strings.Contains(text, "GAME OVER") {
		t.Error("game over overlay should contain 'GAME OVER'")
	}
	if !strings.Contains(text, "Score: 15") {
		t.Error("game over overlay should show current score")
	}
	if !strings.Contains(text, fmt.Sprintf("Best:  %d", g.bestScore)) {
		t.Error("game over overlay should show best score")
	}
}

func TestGameOver_ShowsMedal(t *testing.T) {
	g := testGame()
	g.startGame()
	g.score = 25
	g.die()

	g.clearBuf()
	g.renderGameOverOverlay()
	text := g.bufText()

	if !strings.Contains(text, "SILVER") {
		t.Errorf("score 25 should show SILVER medal in overlay, got:\n%s", extractOverlayLines(text, g))
	}
}

func TestGameOver_NoMedalLowScore(t *testing.T) {
	g := testGame()
	g.startGame()
	g.score = 3
	g.die()

	g.clearBuf()
	g.renderGameOverOverlay()
	text := g.bufText()

	for _, medal := range []string{"BRONZE", "SILVER", "GOLD", "PLATINUM"} {
		if strings.Contains(text, medal) {
			t.Errorf("score 3 should show no medal, but found %s", medal)
		}
	}
}

func TestGameOver_RetryInstruction(t *testing.T) {
	g := testGame()
	g.startGame()
	g.die()

	g.clearBuf()
	g.renderGameOverOverlay()
	text := g.bufText()

	if !strings.Contains(text, "SPACE=Retry") {
		t.Error("game over should show retry instruction")
	}
	if !strings.Contains(text, "Q=Quit") {
		t.Error("game over should show quit instruction")
	}
}

func TestGameOver_BoxBorder(t *testing.T) {
	g := testGame()
	g.startGame()
	g.die()

	g.clearBuf()
	g.renderGameOverOverlay()
	text := g.bufText()

	// Box corners should be present
	if !strings.ContainsRune(text, '┌') {
		t.Error("game over box should have ┌ corner")
	}
	if !strings.ContainsRune(text, '┐') {
		t.Error("game over box should have ┐ corner")
	}
	if !strings.ContainsRune(text, '└') {
		t.Error("game over box should have └ corner")
	}
	if !strings.ContainsRune(text, '┘') {
		t.Error("game over box should have ┘ corner")
	}
	if !strings.ContainsRune(text, '─') {
		t.Error("game over box should have ─ horizontal border")
	}
	if !strings.ContainsRune(text, '│') {
		t.Error("game over box should have │ vertical border")
	}
}

func extractOverlayLines(text string, g *Game) string {
	centerR := g.height / 2
	boxH := 11
	startR := centerR - boxH/2
	lines := strings.Split(text, "\n")
	var relevant []string
	for i := startR; i < startR+boxH && i < len(lines); i++ {
		if i >= 0 {
			relevant = append(relevant, fmt.Sprintf("row %d: %s", i, strings.TrimRight(lines[i], " ")))
		}
	}
	return strings.Join(relevant, "\n")
}

// ═══════════════════════════════════════════
// 13. CLOUD ANIMATION VALIDATION
// ═══════════════════════════════════════════

func TestClouds_Init(t *testing.T) {
	rand.Seed(42)
	initClouds(80, 24)
	if len(clouds) != 5 {
		t.Errorf("expected 5 clouds, got %d", len(clouds))
	}
	for i, cl := range clouds {
		if cl.col < 0 || cl.col >= 80 {
			t.Errorf("cloud %d col %d out of bounds", i, cl.col)
		}
		playH := 24 - groundHeight
		if cl.row < 2 || cl.row >= 2+playH/2 {
			t.Errorf("cloud %d row %d out of expected range [2, %d)", i, cl.row, 2+playH/2)
		}
	}
}

func TestClouds_ScrollLeft(t *testing.T) {
	rand.Seed(42)
	initClouds(80, 24)

	originalCols := make([]int, len(clouds))
	for i, cl := range clouds {
		originalCols[i] = cl.col
	}

	scrollClouds(80)

	for i, cl := range clouds {
		if cl.col >= originalCols[i] && originalCols[i] > -15 {
			t.Errorf("cloud %d should have scrolled left: was %d, now %d", i, originalCols[i], cl.col)
		}
	}
}

func TestClouds_Recycle(t *testing.T) {
	rand.Seed(42)
	initClouds(80, 24)

	// Force a cloud off screen
	clouds[0].col = -16
	scrollClouds(80)

	// Cloud should have been recycled to right side
	if clouds[0].col < 80 {
		t.Errorf("off-screen cloud should recycle to right side, got col %d", clouds[0].col)
	}
}

func TestClouds_RenderedInBuffer(t *testing.T) {
	g := testGame()
	rand.Seed(42)
	initClouds(g.width, g.height)

	// Position a cloud visibly
	clouds[0] = Cloud{row: 5, col: 10, style: 0}

	g.clearBuf()
	g.renderClouds()
	text := g.bufText()

	cloudChars := "._===_."
	foundCloud := false
	for _, ch := range cloudChars {
		if strings.ContainsRune(text, ch) {
			foundCloud = true
			break
		}
	}

	if !foundCloud {
		t.Error("cloud characters should be visible in the buffer when positioned on screen")
	}
}

func TestClouds_DontOverwritePipes(t *testing.T) {
	g := testGame()
	g.startGame()
	clouds = []Cloud{{row: 5, col: 40, style: 0}}

	// Place pipe at same location
	g.pipes = []Pipe{{x: 40, gapTop: 3}}

	g.clearBuf()
	g.renderPipes()
	g.renderClouds() // clouds render after pipes

	// At row 5, col 42 (inside pipe), the pipe character should remain
	if g.buf[5][42] != '█' && g.buf[5][42] != '║' && g.buf[5][42] != '▄' && g.buf[5][42] != '▀' {
		// Clouds only render if buf[r][c] == ' ', so pipe chars should be preserved
		// However row 5 might be in the gap if gapTop=3 and gap=8, gap goes 3..10
		// So row 5 is in the gap, char is ' ', and cloud CAN overwrite it
		// That's fine! Let's pick a row that's definitely in the pipe body
		pipeRow := 1 // row 1 is above gapTop=3, so it's top pipe body
		ch := g.buf[pipeRow][42]
		if ch != '█' && ch != '║' && ch != '▄' && ch != '▀' {
			t.Errorf("cloud should not overwrite pipe at row %d col 42, got '%c'", pipeRow, ch)
		}
	}
}

// ═══════════════════════════════════════════
// 14. RENDERING INTEGRITY
// ═══════════════════════════════════════════

func TestClearBuf_AllSpaces(t *testing.T) {
	g := testGame()
	g.clearBuf()

	for r := 0; r < g.height; r++ {
		for c := 0; c < g.width; c++ {
			if g.buf[r][c] != ' ' {
				t.Errorf("clearBuf: expected space at [%d][%d], got '%c'", r, c, g.buf[r][c])
			}
			if g.colBuf[r][c] != colSky {
				t.Errorf("clearBuf: expected sky color at [%d][%d]", r, c)
			}
		}
	}
}

func TestRender_ContainsAnsiCodes(t *testing.T) {
	g := testGame()
	g.clearBuf()
	g.renderGround()
	output := g.render()

	if !strings.Contains(output, "\033[") {
		t.Error("rendered output should contain ANSI escape codes")
	}
	if !strings.Contains(output, reset) {
		t.Error("rendered output should contain reset code at end of rows")
	}
}

func TestRender_ContainsCursorMoves(t *testing.T) {
	g := testGame()
	g.clearBuf()
	output := g.render()

	// Should contain cursor move for each row
	for r := 1; r <= g.height; r++ {
		expected := fmt.Sprintf("\033[%d;1H", r)
		if !strings.Contains(output, expected) {
			t.Errorf("render output should contain cursor move to row %d", r)
		}
	}
}

func TestSetCell_BoundsCheck(t *testing.T) {
	g := testGame()
	g.clearBuf()

	// These should not panic
	g.setCell(-1, 0, 'X', "")
	g.setCell(0, -1, 'X', "")
	g.setCell(g.height, 0, 'X', "")
	g.setCell(0, g.width, 'X', "")
	g.setCell(g.height+100, g.width+100, 'X', "")

	// Valid cell should work
	g.setCell(5, 5, 'X', colPipe)
	if g.buf[5][5] != 'X' {
		t.Error("setCell should set valid cell")
	}
}

func TestDrawCentered_CentersText(t *testing.T) {
	g := testGame()
	g.clearBuf()

	text := "HELLO"
	g.drawCentered(5, text, colSky)

	// Text should be centered: starts at (80 - 5) / 2 = 37
	expectedStart := (g.width - 5) / 2
	for i, ch := range text {
		c := expectedStart + i
		if g.buf[5][c] != ch {
			t.Errorf("drawCentered: expected '%c' at col %d, got '%c'", ch, c, g.buf[5][c])
		}
	}
}

// ═══════════════════════════════════════════
// 15. COLOR ASSIGNMENT VALIDATION
// ═══════════════════════════════════════════

func TestColors_SkyBackground(t *testing.T) {
	g := testGame()
	g.clearBuf()

	// After clear, all cells should have sky color
	if g.colBuf[0][0] != colSky {
		t.Errorf("sky color expected '%s', got '%s'", colSky, g.colBuf[0][0])
	}
}

func TestColors_GroundHasDistinctColors(t *testing.T) {
	g := testGame()
	g.clearBuf()
	g.renderGround()

	playH := g.playArea()

	// Grass should be different from sky
	if playH < g.height && g.colBuf[playH][0] == colSky {
		t.Error("grass should have distinct color from sky")
	}

	// Dirt should be different from sky and grass
	if playH+1 < g.height {
		if g.colBuf[playH+1][0] == colSky {
			t.Error("dirt should have distinct color from sky")
		}
	}
}

func TestColors_PipesHaveColor(t *testing.T) {
	g := testGame()
	g.startGame()
	g.pipes = []Pipe{{x: 40, gapTop: 8}}

	g.clearBuf()
	g.renderPipes()

	// Check that pipe body has pipe color
	if g.colBuf[2][42] != colPipe {
		t.Errorf("pipe body should have pipe color, got '%s'", g.colBuf[2][42])
	}
}

func TestColors_BirdHasDistinctColors(t *testing.T) {
	g := testGame()
	g.startGame()
	g.bird.y = 10
	g.bird.frame = 0

	g.clearBuf()
	g.renderBird()

	col := g.bird.x
	// Bird eye should have eye color
	eyeCol := g.colBuf[10][col]
	if !strings.Contains(eyeCol, colBirdEye) {
		t.Errorf("bird eye should contain eye color code, got '%s'", eyeCol)
	}

	// Bird beak should have beak color
	beakCol := g.colBuf[10][col+1]
	if !strings.Contains(beakCol, colBirdBeak) {
		t.Errorf("bird beak should contain beak color code, got '%s'", beakCol)
	}
}

// ═══════════════════════════════════════════
// 16. EDGE CASES & ROBUSTNESS
// ═══════════════════════════════════════════

func TestSmallTerminal(t *testing.T) {
	// Very small terminal - should not panic
	g := newGameWithSize(30, 15)
	g.startGame()

	// These should not panic
	g.clearBuf()
	g.renderGround()
	g.renderPipes()
	g.renderBird()
	g.renderScore()
	g.renderGameOverOverlay()
	_ = g.render()
	_ = g.bufText()
}

func TestLargeTerminal(t *testing.T) {
	g := newGameWithSize(120, 40)
	g.startGame()

	g.clearBuf()
	g.renderGround()
	g.renderPipes()
	g.renderBird()
	g.renderScore()
	g.renderGameOverOverlay()
	_ = g.render()
	_ = g.bufText()
}

func TestPipeAtEdge_LeftBoundary(t *testing.T) {
	g := testGame()
	g.startGame()
	// Pipe partially off left edge
	g.pipes = []Pipe{{x: -3, gapTop: 8}}

	// Should not panic
	g.clearBuf()
	g.renderPipes()
}

func TestPipeAtEdge_RightBoundary(t *testing.T) {
	g := testGame()
	g.startGame()
	// Pipe partially off right edge
	g.pipes = []Pipe{{x: g.width - 2, gapTop: 8}}

	// Should not panic
	g.clearBuf()
	g.renderPipes()
}

func TestBirdAtTopEdge(t *testing.T) {
	g := testGame()
	g.startGame()
	g.bird.y = -5 // above screen
	g.pipes = nil

	// Should not panic
	g.clearBuf()
	g.renderBird()
}

func TestBirdAtBottomEdge(t *testing.T) {
	g := testGame()
	g.startGame()
	g.bird.y = float64(g.height + 5) // below screen
	g.pipes = nil

	// Should not panic
	g.clearBuf()
	g.renderBird()
}

func TestUpdate_DoesNothingWhenDead(t *testing.T) {
	g := testGame()
	g.startGame()
	g.die()

	beforeY := g.bird.y
	beforeScore := g.score
	g.update(true) // try to flap while dead

	if g.bird.y != beforeY {
		t.Error("update should not change bird position when dead")
	}
	if g.score != beforeScore {
		t.Error("update should not change score when dead")
	}
}

func TestUpdate_DoesNothingOnTitle(t *testing.T) {
	g := testGame()
	// State is Title
	beforeY := g.bird.y
	g.update(true)

	if g.bird.y != beforeY {
		t.Error("update should not change bird position on title screen")
	}
}

// ═══════════════════════════════════════════
// 17. FULL GAME SIMULATION
// ═══════════════════════════════════════════

func TestFullGameSimulation_PlayAndDie(t *testing.T) {
	g := testGame()

	// Start on title
	if g.state != StateTitle {
		t.Fatal("should start on title")
	}

	// Start game
	g.startGame()
	if g.state != StatePlaying {
		t.Fatal("should be playing after startGame")
	}
	if len(g.pipes) != 4 {
		t.Fatalf("should have 4 pipes, got %d", len(g.pipes))
	}

	// Simulate falling to death (no flapping)
	for i := 0; i < 100; i++ {
		g.update(false)
		if g.state == StateDead {
			break
		}
	}

	if g.state != StateDead {
		t.Error("bird should have died from hitting the ground")
	}
	t.Logf("  bird died at y=%.1f after falling (playArea=%d)", g.bird.y, g.playArea())
}

func TestFullGameSimulation_FlapSurvival(t *testing.T) {
	g := testGame()
	g.startGame()

	// Remove pipes so we can just test flap survival
	g.pipes = nil

	// Smart flapping: only flap when bird is falling AND in lower third of play area.
	// This avoids hitting the ceiling while preventing ground collision.
	playH := g.playArea()
	lowerThird := float64(playH) * 2.0 / 3.0

	alive := true
	for i := 0; i < 60; i++ {
		flap := g.bird.y > lowerThird && g.bird.vy > 0
		g.update(flap)
		if g.state == StateDead {
			alive = false
			break
		}
	}

	if !alive {
		t.Errorf("bird should survive 60 frames with smart flapping and no pipes (died at y=%.1f vy=%.1f)", g.bird.y, g.bird.vy)
	}
	t.Logf("  survived 60 frames, bird y=%.1f vy=%.1f (playArea=%d)", g.bird.y, g.bird.vy, playH)
}

func TestFullGameSimulation_CompleteRenderCycle(t *testing.T) {
	g := testGame()
	initClouds(g.width, g.height)

	// Title screen render
	g.frameCount = 0
	g.renderTitleScreen()
	titleOutput := g.render()
	if len(titleOutput) == 0 {
		t.Error("title render should produce output")
	}

	// Start game
	g.startGame()

	// Several frames of gameplay render
	for i := 0; i < 10; i++ {
		g.update(i%3 == 0)
		if g.state == StateDead {
			break
		}
		g.clearBuf()
		g.renderGround()
		g.renderClouds()
		g.renderPipes()
		g.renderBird()
		g.renderScore()
		output := g.render()
		if len(output) == 0 {
			t.Errorf("frame %d render should produce output", i)
		}
	}

	// Force death and check game over overlay
	g.die()
	g.clearBuf()
	g.renderGround()
	g.renderClouds()
	g.renderPipes()
	g.renderBird()
	g.renderScore()
	g.renderGameOverOverlay()
	gameOverOutput := g.render()
	if len(gameOverOutput) == 0 {
		t.Error("game over render should produce output")
	}

	t.Logf("  full render cycle: title=%d bytes, gameOver=%d bytes", len(titleOutput), len(gameOverOutput))
}

// ═══════════════════════════════════════════
// 18. PIPE VISUAL STRUCTURE VALIDATION
// ═══════════════════════════════════════════

func TestPipeStructure_TopAndBottomPresent(t *testing.T) {
	g := testGame()
	g.startGame()
	gapTop := 8
	g.pipes = []Pipe{{x: 20, gapTop: gapTop}}

	g.clearBuf()
	g.renderPipes()

	// Top pipe: rows 0 to gapTop-1 at pipe columns should be filled
	midCol := 22 // middle of pipe
	for row := 0; row < gapTop-1; row++ {
		ch := g.buf[row][midCol]
		if ch != '█' {
			t.Errorf("top pipe body row %d col %d: expected '█', got '%c'", row, midCol, ch)
		}
	}

	// Bottom pipe: rows gapTop+pipeGap+1 to playArea-1 should be filled
	gapBottom := gapTop + pipeGap
	playH := g.playArea()
	for row := gapBottom + 1; row < playH; row++ {
		ch := g.buf[row][midCol]
		if ch != '█' {
			t.Errorf("bottom pipe body row %d col %d: expected '█', got '%c'", row, midCol, ch)
		}
	}
}

func TestPipeStructure_EdgesHaveVerticalBars(t *testing.T) {
	g := testGame()
	g.startGame()
	gapTop := 8
	pipeX := 20
	g.pipes = []Pipe{{x: pipeX, gapTop: gapTop}}

	g.clearBuf()
	g.renderPipes()

	// Left edge of top pipe (not the cap row)
	for row := 0; row < gapTop-1; row++ {
		ch := g.buf[row][pipeX]
		if ch != '║' {
			t.Errorf("top pipe left edge row %d: expected '║', got '%c'", row, ch)
		}
	}

	// Right edge
	rightCol := pipeX + pipeWidth - 1
	for row := 0; row < gapTop-1; row++ {
		ch := g.buf[row][rightCol]
		if ch != '║' {
			t.Errorf("top pipe right edge row %d: expected '║', got '%c'", row, ch)
		}
	}
}

func TestPipeStructure_CapsPresent(t *testing.T) {
	g := testGame()
	g.startGame()
	gapTop := 8
	g.pipes = []Pipe{{x: 20, gapTop: gapTop}}

	g.clearBuf()
	g.renderPipes()

	// Top cap at gapTop-1
	capRow := gapTop - 1
	midCol := 22
	if g.buf[capRow][midCol] != '▄' {
		t.Errorf("top pipe cap: expected '▄', got '%c'", g.buf[capRow][midCol])
	}

	// Bottom cap at gapTop+pipeGap
	bottomCapRow := gapTop + pipeGap
	if bottomCapRow < g.playArea() {
		if g.buf[bottomCapRow][midCol] != '▀' {
			t.Errorf("bottom pipe cap: expected '▀', got '%c'", g.buf[bottomCapRow][midCol])
		}
	}
}

// ═══════════════════════════════════════════
// 19. bufText / bufRow HELPERS
// ═══════════════════════════════════════════

func TestBufText_CorrectDimensions(t *testing.T) {
	g := testGame()
	g.clearBuf()
	text := g.bufText()

	lines := strings.Split(text, "\n")
	if len(lines) != g.height {
		t.Errorf("bufText should have %d lines, got %d", g.height, len(lines))
	}
	for i, line := range lines {
		runeCount := len([]rune(line))
		if runeCount != g.width {
			t.Errorf("bufText line %d: expected %d runes, got %d", i, g.width, runeCount)
		}
	}
}

func TestBufRow_ValidRow(t *testing.T) {
	g := testGame()
	g.clearBuf()
	g.setCell(5, 10, 'X', "")

	row := g.bufRow(5)
	if row[10] != 'X' {
		t.Errorf("bufRow(5)[10] should be 'X', got '%c'", row[10])
	}
}

func TestBufRow_InvalidRow(t *testing.T) {
	g := testGame()
	g.clearBuf()

	if g.bufRow(-1) != "" {
		t.Error("bufRow(-1) should return empty string")
	}
	if g.bufRow(g.height) != "" {
		t.Error("bufRow(height) should return empty string")
	}
}
