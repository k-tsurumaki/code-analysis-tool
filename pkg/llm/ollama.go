package llm

import (
	"context"
	"encoding/json"
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

type AIAnalysis struct {
	Summary           string   `json:"summary,omitempty"`
	CommentSuggestion string   `json:"comment_suggestion,omitempty"`
	BetterVarNames    []string `json:"better_var_names,omitempty"`
	Improvements      []string `json:"improvements,omitempty"`
}

type Issue struct {
	Kind    string `json:"kind"`
	Pos     string `json:"pos"`
	Message string `json:"message"`
}

func AnalyzeFunction(ctx context.Context, path string, fset *token.FileSet, fn *ast.FuncDecl, issues []Issue, task string) (*AIAnalysis, error) {
	model := getenvDefault("OLLAMA_MODEL", "gpt-oss:20b")
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

	parsed, ok := parseResponse(out)
	if !ok {
		// Return minimal parsed content instead of failing hard
		return &AIAnalysis{Summary: truncate(out, 300)}, nil
	}
	return &parsed, nil
}

type promptData struct {
	Code   string
	Issues []Issue
	Task   string
}

func buildPromptFromFile(code string, issues []Issue, task string) (string, error) {
	// prompt.txtは実行バイナリのカレントディレクトリ or このファイルの親ディレクトリにある想定
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

func extractFuncSource(path string, fset *token.FileSet, fn *ast.FuncDecl) string {
	start := fset.Position(fn.Pos()).Offset
	end := fset.Position(fn.End()).Offset
	bs, _ := os.ReadFile(path)
	if start < 0 || end > len(bs) || start >= end {
		return fn.Name.Name
	}
	return string(bs[start:end])
}

func parseResponse(text string) (AIAnalysis, bool) {
	// Try to locate JSON blob if wrapped in fences
	s := strings.TrimSpace(text)
	if strings.HasPrefix(s, "```") {
		// strip first fence line and last fence if present
		lines := strings.Split(s, "\n")
		if len(lines) >= 2 {
			lines = lines[1:]
			if strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
				lines = lines[:len(lines)-1]
			}
			s = strings.Join(lines, "\n")
		}
	}
	var res AIAnalysis
	if err := json.Unmarshal([]byte(s), &res); err != nil {
		return AIAnalysis{}, false
	}
	return res, true
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func getenvDefault(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
