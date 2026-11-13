# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

herdr のソケット API を叩いて git worktree ごとの Space を管理する、macOS メニューバー常駐アプリ。Swift + SwiftPM。

## Commands

```bash
make test      # swift test（Testing 用オプションとカバレッジ付き）
make app       # build/Yakumo.app を作る
make install   # ~/Applications にコピーする
```

## Layout

- `Sources/YakumoCore` - UI に依存しないロジック（カバレッジ 80% 以上）
- `Sources/Yakumo` - AppKit のエントリーポイントと UI
- `Tests/YakumoCoreTests` - YakumoCore のテスト

## Toolchain Notes

- Command Line Tools だけの環境なので、`swift test` には Makefile の `TEST_FLAGS` が必要。素の `swift test` は使わない。
- KeyboardShortcuts は 1.x（`1.10.0 ..< 1.16.0`）に固定している。1.16 以降と 2.x は `#Preview` を使い、Xcode にしかない PreviewsMacros が必要でビルドできない。

## Go 版の参照

Go 版（tmux + Bubble Tea）は `c88e47e` までに残っている。移植元の参照は `git show c88e47e:internal/github/pr.go` のように読む。
