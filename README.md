# Go Match-3 三消遊戲（不用遊戲引擎）

一個實驗：**完全不用遊戲引擎**，用 Go 標準庫直接呼叫 Win32 API（user32/gdi32）做出有 2D 畫面與基本動畫的三消遊戲。沒有 Ebiten、沒有 SDL、零外部相依 —— 視窗建立、訊息迴圈、雙緩衝繪圖、幀計時、動畫插值全部自己來。

## 執行

```bash
go run .            # GUI 版（Windows）
go run ./cmd/cli    # 終端機文字版（跨平台）
```

## GUI 版玩法

- 滑鼠點選一顆寶石，再點相鄰的寶石交換；連成 3 個以上即消除。
- 無法消除的交換會播放「彈回」動畫，不會生效。
- 消除有縮小動畫；上方寶石以重力加速度掉落補位，新寶石從畫面外落下。
- 連鎖（cascade）自動接續，第 n 波每顆 10×n 分。
- `R` 重新開始，`Esc` 離開；沒有可行步時自動洗牌。

## 沒有引擎時，你要自己寫哪些東西？

| 引擎幫你做的事 | 這裡的對應實作 |
| --- | --- |
| 開視窗、事件迴圈 | `main.go`：`RegisterClassExW`、`CreateWindowExW`、`GetMessage` 訊息幫浦 |
| 渲染器、防閃爍 | `main.go`：GDI 記憶體 DC + `BitBlt` 手工雙緩衝、攔截 `WM_ERASEBKGND` |
| 遊戲迴圈 / 幀計時 | `WM_TIMER`（約 60 FPS）+ 實測 dt，停頓時 clamp 避免動畫跳格 |
| Tween / 動畫系統 | `app.go`：相位狀態機（交換 → 消除 → 掉落 → 連鎖），smoothstep 緩動、`d = ½gt²` 重力掉落 |
| 繪圖 API | `win32.go`：syscall 綁定 `Ellipse` / `Polygon` / `TextOutW` 等 GDI 原語 |
| 輸入處理 | `WM_LBUTTONDOWN` 座標換算格子、`WM_KEYDOWN` |

## 專案結構

```
main.go          Win32 視窗、訊息迴圈、雙緩衝繪製（Windows）
win32.go         user32/gdi32 syscall 綁定與 GDI 小工具
app.go           動畫狀態機、繪圖、輸入（遊戲表現層）
cmd/cli/main.go  終端機文字版
game/board.go    核心邏輯：消除偵測、重力塌落（回傳動畫軌跡）、提示、洗牌
game/board_test.go 單元測試
```

核心設計：`game` package 只負責規則，狀態一步到位；`CollapseAndRefill()` 會回傳每顆寶石「從哪掉到哪」（新寶石從棋盤上方負列開始），表現層拿這份軌跡去插值，就能在「邏輯已完成」與「畫面慢慢演」之間解耦 —— 這也是多數遊戲引擎內部的做法。

## 測試

```bash
go test ./...
```

## 需求

- GUI 版：Windows（Win32/GDI）、Go 1.25+
- CLI 版：任何支援 ANSI 色彩的終端機
