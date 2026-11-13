# Yakumo

macOS メニューバー常駐アプリ。ターミナルマルチプレクサ [herdr](https://github.com/ogulcancelik/herdr) のソケット API を通じて、git worktree ごとの Space の作成・切替・削除を行う。Swift + SwiftPM で構築。

## Requirements

- macOS 14 以降
- Swift 6 ツールチェーン（Command Line Tools のみで可）
- [herdr](https://github.com/ogulcancelik/herdr)（`~/.config/herdr/herdr.sock`）
- [GitHub CLI (`gh`)](https://cli.github.com/) - PR 情報の取得に使う

## Build

```bash
make test      # テスト（カバレッジ付き）
make app       # build/Yakumo.app を作る（ad-hoc 署名）
make install   # ~/Applications/Yakumo.app にコピーする
```

## Configuration

`~/.config/yakumo/config.json`

| フィールド | デフォルト | 説明 |
|---|---|---|
| `default_base_ref` | `origin/main` | worktree 作成の基準にする ref |
| `gh_path` | `/opt/homebrew/bin/gh` | gh の絶対パス |
| `repositories` | `[]` | 管理するリポジトリの一覧 |

ホットキーの既定値は Cmd-G。

## Tech Stack

- Swift 6 / SwiftPM / AppKit
- [KeyboardShortcuts](https://github.com/sindresorhus/KeyboardShortcuts) 1.x
