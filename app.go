//go:build windows

package main

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/fishingboy/go-match3/game"
)

const (
	boardRows = 8
	boardCols = 8
	cellPx    = 64
	hudPx     = 48
	winW      = boardCols * cellPx
	winH      = hudPx + boardRows*cellPx

	swapDur  = 0.15 // seconds for a swap (and swap-back) animation
	clearDur = 0.25 // seconds for the shrink-out of matched gems
	gravity  = 55.0 // fall acceleration, in cells per second squared
)

// Animation phases: the board logic already holds the *final* state of
// each step; the phase only controls how gems are drawn while time t
// runs from 0 to the phase duration.
const (
	phaseIdle = iota
	phaseSwap
	phaseSwapBack
	phaseClear
	phaseFall
)

type App struct {
	board *game.Board
	rng   *rand.Rand
	score int
	chain int // current cascade depth, 0 when settled

	sel   *game.Pos // currently selected cell, nil if none
	phase int
	t     float64 // seconds elapsed in the current phase

	swapA, swapB game.Pos
	clearing     []game.Pos
	falls        []game.Fall
	fallDur      float64
	message      string // transient HUD text (chain bonus, shuffle notice)
}

func newApp(rng *rand.Rand) *App {
	a := &App{rng: rng}
	a.reset()
	return a
}

func (a *App) reset() {
	a.board = game.NewBoard(boardRows, boardCols, a.rng)
	a.score = 0
	a.chain = 0
	a.sel = nil
	a.message = ""
	a.setPhase(phaseIdle)
	a.ensurePlayable()
}

func (a *App) setPhase(p int) {
	a.phase = p
	a.t = 0
}

func (a *App) ensurePlayable() {
	if _, _, ok := a.board.FindHint(); !ok {
		a.board.Shuffle()
		a.message = "No moves left - shuffled!"
	}
}

// click handles a mouse press at pixel (x, y): first click selects a gem,
// a second click on an orthogonal neighbor starts the swap animation.
func (a *App) click(x, y int) {
	if a.phase != phaseIdle {
		return
	}
	c, r := x/cellPx, (y-hudPx)/cellPx
	if y < hudPx || !a.board.InBounds(r, c) {
		a.sel = nil
		return
	}
	p := game.Pos{Row: r, Col: c}
	switch {
	case a.sel == nil:
		a.sel = &p
	case *a.sel == p:
		a.sel = nil
	case game.Adjacent(*a.sel, p):
		a.swapA, a.swapB = *a.sel, p
		a.sel = nil
		a.message = ""
		a.setPhase(phaseSwap)
	default:
		a.sel = &p
	}
}

// update advances the animation state machine by dt seconds.
func (a *App) update(dt float64) {
	a.t += dt
	switch a.phase {
	case phaseSwap:
		if a.t < swapDur {
			return
		}
		a.board.SwapCells(a.swapA, a.swapB)
		if m := a.board.FindMatches(); len(m) > 0 {
			a.chain = 0
			a.startClear(m)
		} else {
			a.board.SwapCells(a.swapA, a.swapB) // revert: invalid move
			a.setPhase(phaseSwapBack)
		}
	case phaseSwapBack:
		if a.t >= swapDur {
			a.setPhase(phaseIdle)
		}
	case phaseClear:
		if a.t < clearDur {
			return
		}
		a.board.Clear(a.clearing)
		a.falls = a.board.CollapseAndRefill()
		a.fallDur = 0
		for _, f := range a.falls {
			if d := fallTime(f); d > a.fallDur {
				a.fallDur = d
			}
		}
		a.setPhase(phaseFall)
	case phaseFall:
		if a.t < a.fallDur {
			return
		}
		if m := a.board.FindMatches(); len(m) > 0 {
			a.startClear(m) // cascade
		} else {
			a.chain = 0
			a.setPhase(phaseIdle)
			a.ensurePlayable()
		}
	}
}

func (a *App) startClear(matches []game.Pos) {
	a.chain++
	a.score += len(matches) * 10 * a.chain
	a.clearing = matches
	if a.chain > 1 {
		a.message = fmt.Sprintf("Chain x%d!", a.chain)
	}
	a.setPhase(phaseClear)
}

// fallTime is how long a constant-acceleration drop over the fall's
// distance takes: d = g*t^2/2  =>  t = sqrt(2d/g).
func fallTime(f game.Fall) float64 {
	return math.Sqrt(2 * float64(f.ToRow-f.FromRow) / gravity)
}

func smoothstep(u float64) float64 {
	if u < 0 {
		u = 0
	}
	if u > 1 {
		u = 1
	}
	return u * u * (3 - 2*u)
}

// cellXY returns the top-left pixel of a board cell.
func cellXY(p game.Pos) (float64, float64) {
	return float64(p.Col * cellPx), float64(hudPx + p.Row*cellPx)
}

var gemColors = map[game.Gem]uint32{
	game.Ruby:     rgb(229, 57, 53),
	game.Emerald:  rgb(67, 160, 71),
	game.Sapphire: rgb(30, 136, 229),
	game.Topaz:    rgb(253, 216, 53),
	game.Amethyst: rgb(171, 71, 188),
	game.Pearl:    rgb(38, 198, 218),
}

var (
	colBG    = rgb(24, 26, 33)
	colCellA = rgb(34, 38, 48)
	colCellB = rgb(41, 46, 58)
	colSel   = rgb(74, 84, 110)
	colHUD   = rgb(15, 17, 22)
	colText  = rgb(230, 233, 240)
	colChain = rgb(253, 216, 53)
)

// draw renders one frame into the (already double-buffered) device context.
func (a *App) draw(dc uintptr) {
	fillRectPx(dc, 0, 0, winW, winH, colBG)

	// Checkerboard cells and selection highlight.
	for r := 0; r < boardRows; r++ {
		for c := 0; c < boardCols; c++ {
			col := colCellA
			if (r+c)%2 == 1 {
				col = colCellB
			}
			if a.sel != nil && a.sel.Row == r && a.sel.Col == c {
				col = colSel
			}
			fillRectPx(dc, c*cellPx, hudPx+r*cellPx, cellPx, cellPx, col)
		}
	}

	a.drawGems(dc)

	// HUD drawn last so gems dropping in from above the board slide out
	// from underneath it instead of overlapping the text.
	fillRectPx(dc, 0, 0, winW, hudPx, colHUD)
	drawText(dc, 12, 10, colText, fmt.Sprintf("Score %d", a.score))
	if a.message != "" {
		drawText(dc, 200, 10, colChain, a.message)
	}
	drawText(dc, winW-205, 12, rgb(120, 128, 145), "R restart  Esc quit")
}

func (a *App) drawGems(dc uintptr) {
	switch a.phase {
	case phaseSwap, phaseSwapBack:
		u := smoothstep(a.t / swapDur)
		if a.phase == phaseSwapBack {
			u = 1 - u
		}
		ax, ay := cellXY(a.swapA)
		bx, by := cellXY(a.swapB)
		for r := 0; r < boardRows; r++ {
			for c := 0; c < boardCols; c++ {
				p := game.Pos{Row: r, Col: c}
				if p == a.swapA || p == a.swapB {
					continue
				}
				x, y := cellXY(p)
				drawGem(dc, a.board.At(r, c), x, y, 1)
			}
		}
		// The two swapping gems glide between their cells.
		drawGem(dc, a.board.At(a.swapA.Row, a.swapA.Col), ax+(bx-ax)*u, ay+(by-ay)*u, 1)
		drawGem(dc, a.board.At(a.swapB.Row, a.swapB.Col), bx+(ax-bx)*u, by+(ay-by)*u, 1)

	case phaseClear:
		shrink := 1 - smoothstep(a.t/clearDur)
		clearSet := map[game.Pos]bool{}
		for _, p := range a.clearing {
			clearSet[p] = true
		}
		for r := 0; r < boardRows; r++ {
			for c := 0; c < boardCols; c++ {
				scale := 1.0
				if clearSet[game.Pos{Row: r, Col: c}] {
					scale = shrink
				}
				x, y := cellXY(game.Pos{Row: r, Col: c})
				drawGem(dc, a.board.At(r, c), x, y, scale)
			}
		}

	case phaseFall:
		// Board already holds the settled state; cells that arrived by
		// falling are drawn at their interpolated in-flight position.
		inFlight := map[game.Pos]game.Fall{}
		for _, f := range a.falls {
			inFlight[game.Pos{Row: f.ToRow, Col: f.Col}] = f
		}
		for r := 0; r < boardRows; r++ {
			for c := 0; c < boardCols; c++ {
				p := game.Pos{Row: r, Col: c}
				x, y := cellXY(p)
				if f, ok := inFlight[p]; ok {
					u := a.t / fallTime(f)
					if u > 1 {
						u = 1
					}
					row := float64(f.FromRow) + float64(f.ToRow-f.FromRow)*u*u
					y = float64(hudPx) + row*cellPx
				}
				drawGem(dc, a.board.At(r, c), x, y, 1)
			}
		}

	default:
		for r := 0; r < boardRows; r++ {
			for c := 0; c < boardCols; c++ {
				x, y := cellXY(game.Pos{Row: r, Col: c})
				drawGem(dc, a.board.At(r, c), x, y, 1)
			}
		}
	}
}

// drawGem paints one gem inside the cell whose top-left pixel is (x, y),
// scaled around the cell center (scale 0 = vanished, 1 = full size).
func drawGem(dc uintptr, g game.Gem, x, y, scale float64) {
	if g == game.Empty || scale <= 0.02 {
		return
	}
	cx, cy := x+cellPx/2, y+cellPx/2
	rad := (cellPx/2 - 9) * scale
	selectBrushPen(dc, gemColors[g])

	switch g {
	case game.Ruby: // circle
		drawEllipse(dc, int(cx-rad), int(cy-rad), int(cx+rad), int(cy+rad))
	case game.Emerald: // rounded square
		drawRoundRect(dc, int(cx-rad), int(cy-rad), int(cx+rad), int(cy+rad), int(rad/2))
	case game.Sapphire: // diamond
		drawPolygon(dc, []point{
			{int32(cx), int32(cy - rad)},
			{int32(cx + rad), int32(cy)},
			{int32(cx), int32(cy + rad)},
			{int32(cx - rad), int32(cy)},
		})
	case game.Topaz: // triangle
		drawPolygon(dc, []point{
			{int32(cx), int32(cy - rad)},
			{int32(cx + rad), int32(cy + rad*0.8)},
			{int32(cx - rad), int32(cy + rad*0.8)},
		})
	case game.Amethyst: // five-pointed star
		pts := make([]point, 10)
		for i := range pts {
			r := rad
			if i%2 == 1 {
				r = rad * 0.45
			}
			ang := -math.Pi/2 + float64(i)*math.Pi/5
			pts[i] = point{int32(cx + r*math.Cos(ang)), int32(cy + r*math.Sin(ang))}
		}
		drawPolygon(dc, pts)
	case game.Pearl: // hexagon
		pts := make([]point, 6)
		for i := range pts {
			ang := -math.Pi/2 + float64(i)*math.Pi/3
			pts[i] = point{int32(cx + rad*math.Cos(ang)), int32(cy + rad*math.Sin(ang))}
		}
		drawPolygon(dc, pts)
	}
}
