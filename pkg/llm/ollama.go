package llm

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

// AIAnalysis はAIによる自然言語の解析結果を格納する構造体です。
type AIAnalysis struct {
	Output string
}

// Issue は静的解析で検出した問題点を表します。
type Issue struct {
	Kind    string `json:"kind"`
	Pos     string `json:"pos"`
	Message string `json:"message"`
}

// AnalyzeFunction は指定したGo関数に対し、AIによるコーディング規約違反の指摘と改善案を取得します。
func AnalyzeFunction(ctx context.Context, path string, fset *token.FileSet, fn *ast.FuncDecl, issues []Issue, task string) (*AIAnalysis, error) {
	model := getenvDefault("OLLAMA_MODEL", "mistral")
	// model := getenvDefault("OLLAMA_MODEL", "gpt-oss:20b")
	client, err := ollama.New(ollama.WithModel(model))
	if err != nil {
		return nil, err
	}

	code := extractFuncSource(path, fset, fn)
	prompt, err := buildPromptFromFile(code, issues, task)
	if err != nil {
		return nil, err
	}

	out, err := llms.GenerateFromSinglePrompt(ctx, client, prompt, llms.WithTemperature(0.2))
	if err != nil {
		return nil, err
	}

	return &AIAnalysis{Output: strings.TrimSpace(out)}, nil
}

type promptData struct {
	Code   string
	Issues []Issue
	Task   string
}

// buildPromptFromFile はprompt.txtテンプレートを読み込み、Goコード・指摘事項・タスク内容を埋め込んだプロンプト文を生成します。
func buildPromptFromFile(code string, issues []Issue, task string) (string, error) {
	promptPath := filepath.Join("prompt.txt")
	bs, err := os.ReadFile(promptPath)
	if err != nil {
		return "", fmt.Errorf("prompt.txtの読み込みに失敗: %w", err)
	}
	tmpl, err := template.New("prompt").Parse(string(bs))
	if err != nil {
		return "", fmt.Errorf("prompt.txtのテンプレートパースに失敗: %w", err)
	}
	var b strings.Builder
	data := promptData{Code: code, Issues: issues, Task: task}
	err = tmpl.Execute(&b, data)
	if err != nil {
		return "", fmt.Errorf("promptテンプレートの埋め込みに失敗: %w", err)
	}
	return b.String(), nil
}

// extractFuncSource は指定した関数のソースコード部分のみをファイルから抽出します。
func extractFuncSource(path string, fset *token.FileSet, fn *ast.FuncDecl) string {
	start := fset.Position(fn.Pos()).Offset
	end := fset.Position(fn.End()).Offset
	bs, _ := os.ReadFile(path)
	if start < 0 || end > len(bs) || start >= end {
		return fn.Name.Name
	}
	return string(bs[start:end])
}

// getenvDefault は環境変数kの値を取得し、未設定ならデフォルト値dを返します。
func getenvDefault(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
