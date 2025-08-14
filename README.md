# GoコードAI支援型分析ツール

## 概要

本ツールはGo言語のソースコードに対し、静的解析（ASTベース）とローカルLLM（Ollama等）を組み合わせて、Goらしいコーディングルールやプロジェクト独自のコーディング規約違反を自動で指摘し、改善案を提案するCLIツールです。新人・上級開発者問わず、ルール遵守の徹底やレビュー効率化を支援します。

## 主な機能

- GoDocコメント不足、パラメータ数超過、未使用変数、error未処理などの静的解析
- LLMによるコーディング規約違反の自動指摘と改善案の提案（出力は日本語の自然文のみ）
- CLIでファイル/ディレクトリ/関数単位で解析・出力
- 出力形式: pretty（標準出力、自然言語のみ）

## セットアップ手順

1. Go 1.20以降をインストール
2. [Ollama](https://ollama.com/)をインストールし、CodeLlama等のモデルを用意
3. 必要なGo依存パッケージを取得
   ```sh
   go get github.com/spf13/cobra@latest
   go get github.com/tmc/langchaingo@latest
   go mod tidy
   ```
4. ビルド
   ```sh
   go build -o gocodeai
   ```

## 使い方

```sh
./gocodeai analyze --file=main.go
```

主なオプション:
- `--file` または `--dir`: 解析対象ファイル/ディレクトリ
- `--func`: 関数名指定（任意）
- `--task`: static(静的解析のみ)|ai(AIのみ)|both(両方)
- `--ai=false`: AI提案を無効化

## 実行例

```sh
./gocodeai analyze --file=main.go
./gocodeai analyze --dir=./internal
```

## 出力例（pretty/自然言語）

```
== sample.go processData ==
- [missing_comment] sample.go:10:1: GoDocコメントがありません
- [too_many_params] sample.go:10:1: パラメータが多い: 5
-- AIによる解析結果 --
この関数はパラメータ数が多すぎます。4つ以下に抑えてください。また、GoDocコメントが不足しています。変数名data, resultは意味が曖昧なので、より具体的な名前に変更しましょう。
```

## 制約・注意事項

- LLM（Ollama等）がローカルで動作する環境が必要です
- AI出力は必ず日本語の自然文のみとなります（JSONやリスト形式は出力されません）
- 大規模・複雑なプロジェクトや特殊な構文には未対応の場合があります
- LLMの出力内容は必ずしも正確・安全とは限りません（プロンプト注入等に注意）
- 初期リリースでは自動修正やWeb連携等は未実装

## 今後の課題

- gofmt/goimports等との連携、自動修正適用
- サードパーティ静的解析ツールとの併用（GoKart等）
- GitHub PR連携やWeb UI対応

## 参考
- [Go公式 go/ast](https://pkg.go.dev/go/ast)
- [langchaingo](https://github.com/tmc/langchaingo)
- [Ollama](https://ollama.com/)
