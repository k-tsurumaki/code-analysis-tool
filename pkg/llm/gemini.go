package llm

import (
	"context"
	"go/ast"
	"go/token"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
)

// AnalyzeFunction は指定したGo関数に対し、AIによるコーディング規約違反の指摘と改善案を取得します。
func AnalyzeFunctionGemini(ctx context.Context, path string, fset *token.FileSet, fn *ast.FuncDecl, issues []Issue, task string) (*AIAnalysis, error) {
	apiKey := getenvDefault("GOOGLE_API_KEY", "")
	model := getenvDefault("GOOGLE_AI_MODEL", "gemini-2.5-flash")
	client, err := googleai.New(ctx, googleai.WithAPIKey(apiKey), googleai.WithDefaultModel(model), googleai.WithDefaultMaxTokens(4096))
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
