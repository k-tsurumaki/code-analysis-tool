package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gocodeai",
		Short: "GoコードAI支援型分析ツール",
		Long:  "GoのAST解析とLLMを用いた自動レビュー支援CLIツール",
	}

	cmd.AddCommand(newAnalyzeCmd())
	return cmd
}

func fail(format string, a ...any) error {
	return fmt.Errorf(format, a...)
}
