# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Git worktreeを管理するためのターミナルUIアプリケーション。Go + Bubble Tea (Charmbracelet) で構築。

## Commands

```bash
# 実行（worktree UI）
go run ./cmd/shiki

# 実行（diff UI）
go run ./cmd/shiki -- --diff

# ビルド
go build -o shiki ./cmd/shiki

# テスト
go test ./...

# テスト（カバレッジ付き）
go test -cover ./...
```

## Architecture

Bubble TeaのElm Architecture (Model-Update-View) パターンに従う。

- `cmd/shiki/main.go` - 統合エントリーポイント（`--diff` フラグでdiff UIに切替）
- `internal/tui/` - worktree UI (Model-Update-View)
- `internal/diffui/` - diff/PR review UI (Model-Update-View)
- `Model` - アプリケーション状態（worktreeリスト、カーソル位置）
- `Update` - キー入力ハンドリング（vim風: j/k, 矢印キー, q/ctrl+c）
- `View` - Lipglossによるスタイル付きレンダリング

## Tech Stack

- Go 1.24
- `charmbracelet/bubbletea` - TUIフレームワーク
- `charmbracelet/lipgloss` - ターミナルスタイリング
