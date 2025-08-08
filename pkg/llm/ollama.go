package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"strings"

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
	model := getenvDefault("OLLAMA_MODEL", "llama3.1:8b")
	client, err := ollama.New(ollama.WithModel(model))
	if err != nil {
		return nil, err
	}

	code := extractFuncSource(path, fset, fn)
	prompt := buildPrompt(code, issues, task)

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

func buildPrompt(code string, issues []Issue, task string) string {
	b := &strings.Builder{}
	fmt.Fprintln(b, "You are a senior Go engineer. Analyze the following function and provide:")
	fmt.Fprintln(b, "- a one-line GoDoc comment if missing")
	fmt.Fprintln(b, "- variable rename suggestions (concise)")
	fmt.Fprintln(b, "- a brief human-readable summary")
	fmt.Fprintln(b, "- concrete improvements for detected anti-patterns")
	fmt.Fprintln(b, "Task:", task)
	fmt.Fprintln(b, "Known issues:")
	for _, is := range issues {
		fmt.Fprintf(b, "- %s: %s\n", is.Kind, is.Message)
	}
	fmt.Fprintln(b, "\nGo function:")
	fmt.Fprintln(b, "```go")
	fmt.Fprintln(b, code)
	fmt.Fprintln(b, "```")
	fmt.Fprintln(b, "\nRespond ONLY valid JSON with keys: summary, comment_suggestion, better_var_names (array), improvements (array). No markdown, no extra text.")
	return b.String()
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
