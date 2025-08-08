# GoコードAI支援型分析ツール

## 概要

本ツールはGo言語のソースコードに対し、静的解析（ASTベース）とローカルLLM（Ollama + CodeLlama等）を組み合わせて、コメント不足検出・変数名リファクタ提案・要約生成・アンチパターン指摘などを自動で行うCLIツールです。新人・上級開発者問わず、コードレビューやドキュメント整備の効率化を支援します。

## 主な機能

- GoDocコメント不足関数の検出
- 5つ以上のパラメータを持つ関数の抽出
- 未使用変数・error未処理の簡易検出
- LLMによるコメント自動生成、変数名リファクタ提案、要約、改善アドバイス
- CLIでファイル/ディレクトリ/関数単位で解析・出力
- 出力形式: pretty(標準出力)/json

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
./gocodeai analyze --file=main.go --task=comment_suggestion
```

主なオプション:
- `--file` または `--dir`: 解析対象ファイル/ディレクトリ
- `--func`: 関数名指定（任意）
- `--task`: comment_suggestion|refactor|summary|anti_pattern|all
- `--output`: pretty|json
- `--ai=false`: AI提案を無効化

## 実行例

```sh
./gocodeai analyze --file=main.go --output=pretty
./gocodeai analyze --dir=./internal --task=all --output=json
```

## 制約・注意事項

- LLM（Ollama/CodeLlama等）がローカルで動作する環境が必要です
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
