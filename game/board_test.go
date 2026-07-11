package game

import (
	"math/rand"
	"testing"
)

func newTestBoard(t *testing.T, layout [][]Gem) *Board {
	t.Helper()
	b := &Board{
		Rows:  len(layout),
		Cols:  len(layout[0]),
		cells: layout,
		rng:   rand.New(rand.NewSource(1)),
	}
	return b
}

func TestNewBoardHasNoInitialMatches(t *testing.T) {
	for seed := int64(0); seed < 50; seed++ {
		b := NewBoard(8, 8, rand.New(rand.NewSource(seed)))
		if m := b.FindMatches(); len(m) != 0 {
			t.Fatalf("seed %d: new board has %d matched cells", seed, len(m))
		}
	}
}

func TestFindMatchesHorizontalAndVertical(t *testing.T) {
	b := newTestBoard(t, [][]Gem{
		{Ruby, Ruby, Ruby, Emerald},
		{Topaz, Pearl, Emerald, Sapphire},
		{Topaz, Emerald, Pearl, Sapphire},
		{Topaz, Pearl, Emerald, Pearl},
	})
	got := map[Pos]bool{}
	for _, p := range b.FindMatches() {
		got[p] = true
	}
	want := []Pos{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {2, 0}, {3, 0}}
	if len(got) != len(want) {
		t.Fatalf("got %d matched cells, want %d: %v", len(got), len(want), got)
	}
	for _, p := range want {
		if !got[p] {
			t.Errorf("expected %v to be matched", p)
		}
	}
}

func TestSwapNotAdjacent(t *testing.T) {
	b := NewBoard(8, 8, rand.New(rand.NewSource(1)))
	if _, _, err := b.Swap(Pos{0, 0}, Pos{0, 2}); err != ErrNotAdjacent {
		t.Fatalf("got %v, want ErrNotAdjacent", err)
	}
	if _, _, err := b.Swap(Pos{0, 0}, Pos{1, 1}); err != ErrNotAdjacent {
		t.Fatalf("diagonal swap: got %v, want ErrNotAdjacent", err)
	}
}

func TestSwapOutOfBounds(t *testing.T) {
	b := NewBoard(4, 4, rand.New(rand.NewSource(1)))
	if _, _, err := b.Swap(Pos{-1, 0}, Pos{0, 0}); err != ErrOutOfBounds {
		t.Fatalf("got %v, want ErrOutOfBounds", err)
	}
}

func TestSwapNoMatchIsReverted(t *testing.T) {
	b := newTestBoard(t, [][]Gem{
		{Ruby, Emerald, Sapphire},
		{Emerald, Sapphire, Ruby},
		{Sapphire, Ruby, Emerald},
	})
	before := [3][3]Gem{}
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			before[r][c] = b.At(r, c)
		}
	}
	if _, _, err := b.Swap(Pos{0, 0}, Pos{0, 1}); err != ErrNoMatch {
		t.Fatalf("got %v, want ErrNoMatch", err)
	}
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			if b.At(r, c) != before[r][c] {
				t.Fatalf("board changed at (%d,%d) after failed swap", r, c)
			}
		}
	}
}

func TestSwapResolvesAndScores(t *testing.T) {
	// Swapping (1,0) with (1,1) lines up three Rubies in column 0.
	b := newTestBoard(t, [][]Gem{
		{Ruby, Pearl, Emerald, Topaz},
		{Emerald, Ruby, Topaz, Pearl},
		{Ruby, Sapphire, Pearl, Emerald},
		{Amethyst, Emerald, Sapphire, Topaz},
	})
	score, cascades, err := b.Swap(Pos{1, 0}, Pos{1, 1})
	if err != nil {
		t.Fatalf("swap failed: %v", err)
	}
	if score < 30 {
		t.Errorf("score = %d, want at least 30", score)
	}
	if cascades < 1 {
		t.Errorf("cascades = %d, want at least 1", cascades)
	}
	if len(b.FindMatches()) != 0 {
		t.Error("board still has matches after Resolve")
	}
}

func TestGravityAndRefill(t *testing.T) {
	b := newTestBoard(t, [][]Gem{
		{Topaz, Empty, Pearl},
		{Empty, Empty, Empty},
		{Empty, Sapphire, Ruby},
	})
	b.applyGravity()
	if b.At(2, 0) != Topaz {
		t.Errorf("Topaz should fall to bottom of column 0, got %v", b.At(2, 0))
	}
	if b.At(2, 1) != Sapphire {
		t.Errorf("Sapphire should stay at bottom of column 1, got %v", b.At(2, 1))
	}
	if b.At(1, 2) != Pearl || b.At(2, 2) != Ruby {
		t.Errorf("column 2 should be [Empty, Pearl, Ruby], got [%v, %v, %v]",
			b.At(0, 2), b.At(1, 2), b.At(2, 2))
	}
	b.refill()
	for r := 0; r < b.Rows; r++ {
		for c := 0; c < b.Cols; c++ {
			if b.At(r, c) == Empty {
				t.Fatalf("cell (%d,%d) still empty after refill", r, c)
			}
		}
	}
}

func TestFindHintOnFreshBoard(t *testing.T) {
	// A fresh 8x8 board almost always has a move; Shuffle guarantees one.
	b := NewBoard(8, 8, rand.New(rand.NewSource(7)))
	b.Shuffle()
	a, c, ok := b.FindHint()
	if !ok {
		t.Fatal("no hint found after Shuffle")
	}
	if _, _, err := b.Swap(a, c); err != nil {
		t.Fatalf("hinted swap %v<->%v failed: %v", a, c, err)
	}
}

func TestCascadeScoringMultiplier(t *testing.T) {
	// First cascade: 3 gems * 10 * 1 = 30. Any extra cascades only add more.
	b := newTestBoard(t, [][]Gem{
		{Ruby, Ruby, Sapphire},
		{Pearl, Emerald, Ruby},
		{Emerald, Pearl, Topaz},
	})
	score, _, err := b.Swap(Pos{0, 2}, Pos{1, 2})
	if err != nil {
		t.Fatalf("swap failed: %v", err)
	}
	if score < 30 {
		t.Errorf("score = %d, want >= 30", score)
	}
}
