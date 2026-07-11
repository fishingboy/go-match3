// Command go-match3 is an interactive terminal match-3 game.
//
// Swap two adjacent gems to line up 3 or more of the same kind.
// Cleared gems score points, gems above fall down, and chain
// reactions multiply your score.
package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/fishingboy/go-match3/game"
)

const (
	boardRows  = 8
	boardCols  = 8
	totalMoves = 20
)

var gemFaces = map[game.Gem]string{
	game.Empty:    "  ",
	game.Ruby:     "\x1b[31m●\x1b[0m ",
	game.Emerald:  "\x1b[32m■\x1b[0m ",
	game.Sapphire: "\x1b[34m◆\x1b[0m ",
	game.Topaz:    "\x1b[33m▲\x1b[0m ",
	game.Amethyst: "\x1b[35m★\x1b[0m ",
	game.Pearl:    "\x1b[36m♥\x1b[0m ",
}

func main() {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	board := game.NewBoard(boardRows, boardCols, rng)
	score := 0
	movesLeft := totalMoves

	fmt.Println("=== Go Match-3 ===")
	fmt.Println("指令: 交換兩個相鄰寶石，例如 `a1 a2`；`hint` 提示；`q` 離開")

	scanner := bufio.NewScanner(os.Stdin)
	for movesLeft > 0 {
		ensurePlayable(board)
		render(board, score, movesLeft)

		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(strings.ToLower(scanner.Text()))
		switch {
		case line == "q" || line == "quit" || line == "exit":
			movesLeft = 0
		case line == "hint" || line == "h":
			if a, c, ok := board.FindHint(); ok {
				fmt.Printf("提示: %s %s\n", posToCoord(a), posToCoord(c))
			}
		case line == "":
			// ignore empty input
		default:
			a, c, err := parseMove(line)
			if err != nil {
				fmt.Println("看不懂的指令，格式範例: a1 a2")
				continue
			}
			gained, cascades, err := board.Swap(a, c)
			switch err {
			case nil:
				score += gained
				movesLeft--
				if cascades > 1 {
					fmt.Printf("連鎖 x%d！+%d 分\n", cascades, gained)
				} else {
					fmt.Printf("+%d 分\n", gained)
				}
			case game.ErrNoMatch:
				fmt.Println("這一步不會消除任何寶石，換一步吧")
			case game.ErrNotAdjacent:
				fmt.Println("只能交換上下左右相鄰的寶石")
			default:
				fmt.Println("無效的位置")
			}
		}
	}

	fmt.Printf("\n遊戲結束！總分: %d\n", score)
}

// ensurePlayable reshuffles the board when no valid move remains.
func ensurePlayable(b *game.Board) {
	if _, _, ok := b.FindHint(); !ok {
		fmt.Println("沒有可消除的步了，重新洗牌...")
		b.Shuffle()
	}
}

func render(b *game.Board, score, movesLeft int) {
	fmt.Printf("\n分數: %d   剩餘步數: %d\n\n   ", score, movesLeft)
	for c := 0; c < b.Cols; c++ {
		fmt.Printf("%c ", 'a'+c)
	}
	fmt.Println()
	for r := 0; r < b.Rows; r++ {
		fmt.Printf("%2d ", r+1)
		for c := 0; c < b.Cols; c++ {
			fmt.Print(gemFaces[b.At(r, c)])
		}
		fmt.Println()
	}
	fmt.Println()
}

// parseMove parses input like "a1 b1" into two board positions.
func parseMove(line string) (game.Pos, game.Pos, error) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return game.Pos{}, game.Pos{}, fmt.Errorf("expected two coordinates")
	}
	a, err := parseCoord(fields[0])
	if err != nil {
		return game.Pos{}, game.Pos{}, err
	}
	c, err := parseCoord(fields[1])
	if err != nil {
		return game.Pos{}, game.Pos{}, err
	}
	return a, c, nil
}

// parseCoord parses a coordinate like "a1" (column letter + 1-based row).
func parseCoord(s string) (game.Pos, error) {
	if len(s) < 2 || s[0] < 'a' || s[0] > 'z' {
		return game.Pos{}, fmt.Errorf("bad coordinate %q", s)
	}
	col := int(s[0] - 'a')
	row := 0
	for _, ch := range s[1:] {
		if ch < '0' || ch > '9' {
			return game.Pos{}, fmt.Errorf("bad coordinate %q", s)
		}
		row = row*10 + int(ch-'0')
	}
	if row == 0 {
		return game.Pos{}, fmt.Errorf("bad coordinate %q", s)
	}
	return game.Pos{Row: row - 1, Col: col}, nil
}

func posToCoord(p game.Pos) string {
	return fmt.Sprintf("%c%d", 'a'+p.Col, p.Row+1)
}
