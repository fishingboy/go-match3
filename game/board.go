// Package game implements the core logic of a match-3 puzzle game:
// board generation, swapping, match detection, gravity and cascading.
package game

import (
	"errors"
	"math/rand"
)

// Gem is a single tile on the board. Empty (0) marks a hole waiting to be filled.
type Gem int

const (
	Empty Gem = iota
	Ruby
	Emerald
	Sapphire
	Topaz
	Amethyst
	Pearl
)

// NumGemTypes is the number of distinct (non-empty) gem kinds.
const NumGemTypes = 6

// Pos is a cell coordinate on the board (0-based).
type Pos struct {
	Row, Col int
}

// Board holds the game grid and its random source.
type Board struct {
	Rows, Cols int
	cells      [][]Gem
	rng        *rand.Rand
}

// ErrNotAdjacent is returned when the two cells of a swap are not orthogonal neighbors.
var ErrNotAdjacent = errors.New("cells are not adjacent")

// ErrNoMatch is returned when a swap would not create any match; the board is unchanged.
var ErrNoMatch = errors.New("swap creates no match")

// ErrOutOfBounds is returned when a coordinate is outside the board.
var ErrOutOfBounds = errors.New("position out of bounds")

// NewBoard creates a rows x cols board filled with random gems,
// guaranteed to start with no pre-existing matches.
func NewBoard(rows, cols int, rng *rand.Rand) *Board {
	b := &Board{Rows: rows, Cols: cols, rng: rng}
	b.cells = make([][]Gem, rows)
	for r := range b.cells {
		b.cells[r] = make([]Gem, cols)
	}
	b.fillWithoutMatches()
	return b
}

// At returns the gem at (r, c).
func (b *Board) At(r, c int) Gem {
	return b.cells[r][c]
}

// InBounds reports whether (r, c) is on the board.
func (b *Board) InBounds(r, c int) bool {
	return r >= 0 && r < b.Rows && c >= 0 && c < b.Cols
}

func (b *Board) fillWithoutMatches() {
	for r := 0; r < b.Rows; r++ {
		for c := 0; c < b.Cols; c++ {
			b.cells[r][c] = b.randomGemAvoiding(r, c)
		}
	}
}

// randomGemAvoiding picks a gem that does not complete a horizontal or
// vertical run of 3 with the already-filled cells above and to the left.
func (b *Board) randomGemAvoiding(r, c int) Gem {
	for {
		g := Gem(b.rng.Intn(NumGemTypes) + 1)
		if c >= 2 && b.cells[r][c-1] == g && b.cells[r][c-2] == g {
			continue
		}
		if r >= 2 && b.cells[r-1][c] == g && b.cells[r-2][c] == g {
			continue
		}
		return g
	}
}

// Swap exchanges two adjacent gems. If the swap creates no match it is
// reverted and ErrNoMatch is returned. On success it returns the score
// gained after all cascades settle and the number of cascade steps.
func (b *Board) Swap(a, c Pos) (score, cascades int, err error) {
	if !b.InBounds(a.Row, a.Col) || !b.InBounds(c.Row, c.Col) {
		return 0, 0, ErrOutOfBounds
	}
	if !Adjacent(a, c) {
		return 0, 0, ErrNotAdjacent
	}
	b.SwapCells(a, c)
	if len(b.FindMatches()) == 0 {
		b.SwapCells(a, c)
		return 0, 0, ErrNoMatch
	}
	score, cascades = b.Resolve()
	return score, cascades, nil
}

// SwapCells exchanges two cells unconditionally, without validation or
// match resolution. UIs that animate the swap themselves use this
// together with FindMatches, Clear and CollapseAndRefill.
func (b *Board) SwapCells(a, c Pos) {
	b.cells[a.Row][a.Col], b.cells[c.Row][c.Col] = b.cells[c.Row][c.Col], b.cells[a.Row][a.Col]
}

// Adjacent reports whether two positions are orthogonal neighbors.
func Adjacent(a, c Pos) bool {
	dr, dc := a.Row-c.Row, a.Col-c.Col
	if dr < 0 {
		dr = -dr
	}
	if dc < 0 {
		dc = -dc
	}
	return dr+dc == 1
}

// FindMatches returns every cell that is part of a horizontal or vertical
// run of 3 or more identical gems.
func (b *Board) FindMatches() []Pos {
	matched := make(map[Pos]bool)

	// Horizontal runs.
	for r := 0; r < b.Rows; r++ {
		runStart := 0
		for c := 1; c <= b.Cols; c++ {
			if c < b.Cols && b.cells[r][c] != Empty && b.cells[r][c] == b.cells[r][runStart] {
				continue
			}
			if c-runStart >= 3 && b.cells[r][runStart] != Empty {
				for k := runStart; k < c; k++ {
					matched[Pos{r, k}] = true
				}
			}
			runStart = c
		}
	}

	// Vertical runs.
	for c := 0; c < b.Cols; c++ {
		runStart := 0
		for r := 1; r <= b.Rows; r++ {
			if r < b.Rows && b.cells[r][c] != Empty && b.cells[r][c] == b.cells[runStart][c] {
				continue
			}
			if r-runStart >= 3 && b.cells[runStart][c] != Empty {
				for k := runStart; k < r; k++ {
					matched[Pos{k, c}] = true
				}
			}
			runStart = r
		}
	}

	out := make([]Pos, 0, len(matched))
	for p := range matched {
		out = append(out, p)
	}
	return out
}

// Resolve repeatedly clears matches, drops gems and refills until the board
// is stable. Each gem cleared in cascade step n scores 10*n points, so
// chain reactions are worth more.
func (b *Board) Resolve() (score, cascades int) {
	for {
		matches := b.FindMatches()
		if len(matches) == 0 {
			return score, cascades
		}
		cascades++
		score += len(matches) * 10 * cascades
		b.Clear(matches)
		b.CollapseAndRefill()
	}
}

// Clear empties the given cells (typically the result of FindMatches).
func (b *Board) Clear(cells []Pos) {
	for _, p := range cells {
		b.cells[p.Row][p.Col] = Empty
	}
}

// Fall describes one gem's vertical drop after matches are cleared:
// the gem in column Col falls from FromRow to ToRow. Newly spawned gems
// start above the board, so their FromRow is negative.
type Fall struct {
	Col, FromRow, ToRow int
	Gem                 Gem
}

// CollapseAndRefill slides gems down into empty cells and spawns new gems
// at the top, returning every movement so a UI can animate the drops.
// Existing-gem falls are reported before spawns within each column.
func (b *Board) CollapseAndRefill() []Fall {
	var falls []Fall
	for c := 0; c < b.Cols; c++ {
		write := b.Rows - 1
		for r := b.Rows - 1; r >= 0; r-- {
			if b.cells[r][c] == Empty {
				continue
			}
			if write != r {
				g := b.cells[r][c]
				b.cells[write][c] = g
				b.cells[r][c] = Empty
				falls = append(falls, Fall{Col: c, FromRow: r, ToRow: write, Gem: g})
			}
			write--
		}
		// Rows 0..write are now empty; spawn replacements stacked above
		// the board so they visually drop in from off-screen.
		spawnCount := write + 1
		for r := write; r >= 0; r-- {
			g := Gem(b.rng.Intn(NumGemTypes) + 1)
			b.cells[r][c] = g
			falls = append(falls, Fall{Col: c, FromRow: r - spawnCount, ToRow: r, Gem: g})
		}
	}
	return falls
}

// FindHint returns one valid move (a pair of cells whose swap creates a
// match), or ok=false when no move exists and the board needs a shuffle.
func (b *Board) FindHint() (a, c Pos, ok bool) {
	dirs := []Pos{{0, 1}, {1, 0}}
	for r := 0; r < b.Rows; r++ {
		for col := 0; col < b.Cols; col++ {
			for _, d := range dirs {
				nr, nc := r+d.Row, col+d.Col
				if !b.InBounds(nr, nc) {
					continue
				}
				p1, p2 := Pos{r, col}, Pos{nr, nc}
				b.SwapCells(p1, p2)
				found := len(b.FindMatches()) > 0
				b.SwapCells(p1, p2)
				if found {
					return p1, p2, true
				}
			}
		}
	}
	return Pos{}, Pos{}, false
}

// Shuffle regenerates the board (no pre-existing matches) until at least
// one valid move exists. Used when the player runs out of moves.
func (b *Board) Shuffle() {
	for {
		b.fillWithoutMatches()
		if _, _, ok := b.FindHint(); ok {
			return
		}
	}
}
