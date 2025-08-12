package cmd

import (
	"context"
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

// analyzeOptions はコマンドライン引数のオプションを格納する構造体です。
type analyzeOptions struct {
	Path     string
	FuncName string
	Task     string
	WithAI   bool
}

// AnalyzeResult は1関数ごとの解析結果を表します。
type AnalyzeResult struct {
	File     string  `json:"file"`
	Function string  `json:"function,omitempty"`
	Issues   []Issue `json:"issues"`
	AIOutput string  `json:"ai_output,omitempty"`
}

// Issue は静的解析で検出した問題点を表します。
type Issue struct {
	Kind    string `json:"kind"`
	Pos     string `json:"pos"`
	Message string `json:"message"`
}

// AIAnalysis はAIによる自然言語の解析結果を格納します。
type AIAnalysis struct {
	Output string
}

// newAnalyzeCmd は analyze サブコマンドを生成します。
func newAnalyzeCmd() *cobra.Command {
	opt := &analyzeOptions{}
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Goコードを解析しAIでコーディング規約違反を指摘",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opt.Path == "" {
				return fail("--file または --dir を指定してください")
			}
			if opt.Task != "static" && opt.Task != "ai" && opt.Task != "both" {
				return fail("--task は static, ai, both のいずれかを指定してください")
			}
			results, err := runAnalysis(cmd.Context(), opt)
			if err != nil {
				return err
			}
			for _, r := range results {
				fmt.Printf("\n== %s %s ==\n", r.File, r.Function)
				if opt.Task != "ai" {
					for _, is := range r.Issues {
						fmt.Printf("- [%s] %s: %s\n", is.Kind, is.Pos, is.Message)
					}
				}
				if opt.Task != "static" && r.AIOutput != "" {
					fmt.Println("-- AIによる解析結果 --")
					fmt.Println(r.AIOutput)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&opt.Path, "file", "", "解析対象のGoファイルまたはディレクトリ")
	cmd.Flags().StringVar(&opt.Path, "dir", "", "解析対象ディレクトリ (fileと排他)")
	cmd.Flags().StringVar(&opt.FuncName, "func", "", "特定の関数名に限定して解析")
	cmd.Flags().StringVar(&opt.Task, "task", "both", "static(静的解析のみ)|ai(AIのみ)|both(両方)")
	cmd.Flags().BoolVar(&opt.WithAI, "ai", true, "AI提案を有効化")

	return cmd
}

// runAnalysis は指定されたファイルまたはディレクトリを解析し、各関数ごとの結果を返します。
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

// analyzeFile は1ファイル内の各関数をASTで解析し、静的解析・AI解析結果を返します。
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
		if opt.Task != "ai" {
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
		}

		res := AnalyzeResult{File: path, Function: fn.Name.Name, Issues: issues}
		if opt.WithAI && opt.Task != "static" {
			llmIssues := make([]llm.Issue, 0, len(issues))
			if opt.Task == "ai" {
				// 静的解析結果をAIに渡さず、空で呼ぶ
				llmIssues = nil
			} else {
				for _, is := range issues {
					llmIssues = append(llmIssues, llm.Issue{Kind: is.Kind, Pos: is.Pos, Message: is.Message})
				}
			}
			ai, err := llm.AnalyzeFunction(ctx, path, fset, fn, llmIssues, opt.Task)
			if err != nil {
				res.Issues = append(res.Issues, Issue{Kind: "ai_error", Pos: "-", Message: err.Error()})
			} else if ai != nil {
				res.AIOutput = ai.Output
			}
		}
		results = append(results, res)
		return true
	})
	return results, nil
}
