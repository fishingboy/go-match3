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
	if !adjacent(a, c) {
		return 0, 0, ErrNotAdjacent
	}
	b.swapCells(a, c)
	if len(b.FindMatches()) == 0 {
		b.swapCells(a, c)
		return 0, 0, ErrNoMatch
	}
	score, cascades = b.Resolve()
	return score, cascades, nil
}

func (b *Board) swapCells(a, c Pos) {
	b.cells[a.Row][a.Col], b.cells[c.Row][c.Col] = b.cells[c.Row][c.Col], b.cells[a.Row][a.Col]
}

func adjacent(a, c Pos) bool {
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
		for _, p := range matches {
			b.cells[p.Row][p.Col] = Empty
		}
		b.applyGravity()
		b.refill()
	}
}

// applyGravity slides gems down into empty cells, column by column.
func (b *Board) applyGravity() {
	for c := 0; c < b.Cols; c++ {
		write := b.Rows - 1
		for r := b.Rows - 1; r >= 0; r-- {
			if b.cells[r][c] != Empty {
				b.cells[write][c] = b.cells[r][c]
				write--
			}
		}
		for r := write; r >= 0; r-- {
			b.cells[r][c] = Empty
		}
	}
}

// refill fills every empty cell with a random gem.
func (b *Board) refill() {
	for r := 0; r < b.Rows; r++ {
		for c := 0; c < b.Cols; c++ {
			if b.cells[r][c] == Empty {
				b.cells[r][c] = Gem(b.rng.Intn(NumGemTypes) + 1)
			}
		}
	}
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
				b.swapCells(p1, p2)
				found := len(b.FindMatches()) > 0
				b.swapCells(p1, p2)
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
