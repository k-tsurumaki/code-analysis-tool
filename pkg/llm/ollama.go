package llm

import (
	"context"
	"go/ast"
	"go/token"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

// AnalyzeFunction は指定したGo関数に対し、AIによるコーディング規約違反の指摘と改善案を取得します。
func AnalyzeFunctionOllama(ctx context.Context, path string, fset *token.FileSet, fn *ast.FuncDecl, issues []Issue, task string) (*AIAnalysis, error) {
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

	out, err := llms.GenerateFromSinglePrompt(ctx, client, prompt, llms.WithTemperature(0.2), llms.WithMaxTokens(4096))
	if err != nil {
		return nil, err
	}

	return &AIAnalysis{Output: strings.TrimSpace(out)}, nil
}
