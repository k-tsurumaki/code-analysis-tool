package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/k-tsurumaki/code-analysis-tool/pkg/llm"
)

type analyzeOptions struct {
	Path     string
	FuncName string
	Task     string
	Output   string
	WithAI   bool
}

type AnalyzeResult struct {
	File     string      `json:"file"`
	Function string      `json:"function,omitempty"`
	Issues   []Issue     `json:"issues"`
	AI       *AIAnalysis `json:"ai,omitempty"`
}

type Issue struct {
	Kind    string `json:"kind"`
	Pos     string `json:"pos"`
	Message string `json:"message"`
}

type AIAnalysis struct {
	Summary           string   `json:"summary,omitempty"`
	CommentSuggestion string   `json:"comment_suggestion,omitempty"`
	BetterVarNames    []string `json:"better_var_names,omitempty"`
	Improvements      []string `json:"improvements,omitempty"`
}

func newAnalyzeCmd() *cobra.Command {
	opt := &analyzeOptions{}
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Goコードを解析しAIで提案を生成",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opt.Path == "" {
				return fail("--file または --dir を指定してください")
			}
			results, err := runAnalysis(cmd.Context(), opt)
			if err != nil {
				return err
			}
			switch strings.ToLower(opt.Output) {
			case "json":
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			default:
				for _, r := range results {
					fmt.Printf("\n== %s %s ==\n", r.File, r.Function)
					for _, is := range r.Issues {
						fmt.Printf("- [%s] %s: %s\n", is.Kind, is.Pos, is.Message)
					}
					if r.AI != nil {
						fmt.Println("-- AI Suggestions --")
						if r.AI.Summary != "" {
							fmt.Println("Summary:", r.AI.Summary)
						}
						if r.AI.CommentSuggestion != "" {
							fmt.Println("Comment:", r.AI.CommentSuggestion)
						}
						if len(r.AI.BetterVarNames) > 0 {
							fmt.Println("Better Var Names:")
							for _, v := range r.AI.BetterVarNames {
								fmt.Println("  -", v)
							}
						}
						if len(r.AI.Improvements) > 0 {
							fmt.Println("Improvements:")
							for _, v := range r.AI.Improvements {
								fmt.Println("  -", v)
							}
						}
					}
				}
				return nil
			}
		},
	}

	cmd.Flags().StringVar(&opt.Path, "file", "", "解析対象のGoファイルまたはディレクトリ")
	cmd.Flags().StringVar(&opt.Path, "dir", "", "解析対象ディレクトリ (fileと排他)")
	cmd.Flags().StringVar(&opt.FuncName, "func", "", "特定の関数名に限定して解析")
	cmd.Flags().StringVar(&opt.Task, "task", "all", "comment_suggestion|refactor|summary|anti_pattern|all")
	cmd.Flags().StringVar(&opt.Output, "output", "pretty", "pretty|json")
	cmd.Flags().BoolVar(&opt.WithAI, "ai", true, "AI提案を有効化")

	return cmd
}

func runAnalysis(ctx context.Context, opt *analyzeOptions) ([]AnalyzeResult, error) {
	files := []string{}
	info, err := os.Stat(opt.Path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		err = filepath.WalkDir(opt.Path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		files = append(files, opt.Path)
	}

	results := []AnalyzeResult{}
	for _, f := range files {
		ar, err := analyzeFile(ctx, f, opt)
		if err != nil {
			return nil, err
		}
		results = append(results, ar...)
	}
	return results, nil
}

func analyzeFile(ctx context.Context, path string, opt *analyzeOptions) ([]AnalyzeResult, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	results := []AnalyzeResult{}
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}
		if opt.FuncName != "" && fn.Name.Name != opt.FuncName {
			return true
		}

		issues := []Issue{}
		// コメント不足
		if fn.Doc == nil || len(fn.Doc.List) == 0 {
			pos := fset.Position(fn.Pos())
			issues = append(issues, Issue{Kind: "missing_comment", Pos: pos.String(), Message: "GoDocコメントがありません"})
		}
		// パラメータ多すぎ
		if fn.Type.Params != nil && fn.Type.Params.NumFields() >= 1 {
			count := 0
			for _, f := range fn.Type.Params.List {
				names := 1
				if len(f.Names) > 0 {
					names = len(f.Names)
				}
				count += names
			}
			if count >= 5 {
				pos := fset.Position(fn.Pos())
				issues = append(issues, Issue{Kind: "too_many_params", Pos: pos.String(), Message: fmt.Sprintf("パラメータが多い: %d", count)})
			}
		}
		// 未使用変数の簡易検出 ("_"以外でIdentかつ使用されてない可能性) - 簡易版
		used := map[string]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			used[id.Name] = true
			return true
		})
		if fn.Body != nil {
			for _, stmt := range fn.Body.List {
				assign, ok := stmt.(*ast.AssignStmt)
				if !ok {
					continue
				}
				for _, lh := range assign.Lhs {
					ident, ok := lh.(*ast.Ident)
					if !ok {
						continue
					}
					if ident.Name == "_" {
						continue
					}
					if !used[ident.Name] {
						pos := fset.Position(ident.Pos())
						issues = append(issues, Issue{Kind: "unused_var", Pos: pos.String(), Message: fmt.Sprintf("未使用変数の可能性: %s", ident.Name)})
					}
				}
			}
		}
		// error未処理の簡易検出: 識別子名がerrの割当後に直後で使用無いケースの超簡易版
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, rh := range assign.Rhs {
				_ = i
				_ = rh
				// 実装を簡略化: シンボル名"err"が左辺に現れたら警告
				for _, lh := range assign.Lhs {
					if ident, ok := lh.(*ast.Ident); ok && ident.Name == "err" {
						pos := fset.Position(ident.Pos())
						issues = append(issues, Issue{Kind: "unhandled_error", Pos: pos.String(), Message: "errorの処理を確認してください"})
					}
				}
			}
			return true
		})

		res := AnalyzeResult{File: path, Function: fn.Name.Name, Issues: issues}
		if opt.WithAI {
			// Convert to llm.Issue
			llmIssues := make([]llm.Issue, 0, len(issues))
			for _, is := range issues {
				llmIssues = append(llmIssues, llm.Issue{Kind: is.Kind, Pos: is.Pos, Message: is.Message})
			}
			ai, err := llm.AnalyzeFunction(ctx, path, fset, fn, llmIssues, opt.Task)
			if err != nil {
				res.Issues = append(res.Issues, Issue{Kind: "ai_error", Pos: "-", Message: err.Error()})
			} else if ai != nil {
				res.AI = &AIAnalysis{Summary: ai.Summary, CommentSuggestion: ai.CommentSuggestion, BetterVarNames: ai.BetterVarNames, Improvements: ai.Improvements}
			}
		}
		results = append(results, res)
		return true
	})
	return results, nil
}
