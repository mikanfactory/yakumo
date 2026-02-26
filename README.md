# Yakumo

Git worktree を管理するターミナル UI アプリケーション。Go + Bubble Tea (Charmbracelet) で構築。tmux 上でワークツリーの作成・切替・削除を行い、GitHub PR レビューや Claude Code エージェント検知を統合した開発環境を提供する。

## Features

- **Worktree 管理** - ワークツリーの一覧表示・作成・削除を TUI で操作
- **GitHub 連携** - PR/ブランチ URL からのワークツリー作成、PR レビュー UI（CI チェック・コメント・マージ状態の表示）
- **tmux セッション自動構築** - ワークツリー選択時にメインウィンドウ + バックグラウンドウィンドウのペインレイアウトを自動作成
- **Claude Code 統合** - エージェント状態のリアルタイム検知（Idle / Running / Waiting）、LLM によるブランチ名自動生成
- **ペインスワップ** - メインウィンドウとバックグラウンドウィンドウ間でペイン内容を入れ替え
- **Vim 風キーバインド** - `j/k`、矢印キー、`enter`、`d`（削除）、`q`（終了）

## Requirements

- [Go](https://go.dev/) 1.24+
- [tmux](https://github.com/tmux/tmux)
- [GitHub CLI (`gh`)](https://cli.github.com/) - PR 連携に必要（オプション）
- [Claude CLI (`claude`)](https://docs.anthropic.com/en/docs/claude-code) - ブランチ名自動生成に必要（オプション）

## Installation

```bash
# ビルド
go build -o yakumo ./cmd/yakumo

# パスの通った場所に配置
mv yakumo /usr/local/bin/
```

## Usage

```bash
# Worktree UI を起動
yakumo

# カスタム設定ファイルを指定
yakumo --config /path/to/config.yaml

# Diff/PR レビュー UI を起動
yakumo diff-ui

# センターペインをスワップ
yakumo swap-center

# 右下ペインをスワップ
yakumo swap-right-below
```

## Configuration

設定ファイルは `~/.config/yakumo/config.yaml` に配置する。初回起動時に自動生成される。

```yaml
sidebar_width: 30
default_base_ref: origin/main
worktree_base_path: ~/yakumo

repositories:
  - name: yakumo
    path: /Users/you/code/yakumo
    startup_command: "tmux send-keys -t pane 'nvim' Enter"
    rb_commands:
      - "make test"
      - "npm run lint"
      - "git push"
```

| フィールド | デフォルト | 説明 |
|---|---|---|
| `sidebar_width` | `30` | サイドバーの幅 |
| `default_base_ref` | `origin/main` | 差分計算や worktree 作成の基準に使う ref |
| `worktree_base_path` | `~/yakumo` | ワークツリーを作成するベースパス |
| `repositories` | (必須) | 管理するリポジトリの一覧 |
| `repositories[].name` | | リポジトリの表示名 |
| `repositories[].path` | | リポジトリのパス |
| `repositories[].startup_command` | | セッション作成時に実行するコマンド（オプション） |
| `repositories[].rb_commands` | | 右下ペインで実行するコマンド一覧（最大 3 つ、オプション） |

## Tech Stack

- [Go](https://go.dev/) 1.24
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI フレームワーク
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - ターミナルスタイリング
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI コンポーネント
- [BubbleZone](https://github.com/lrstanley/bubblezone) - マウスゾーンサポート
