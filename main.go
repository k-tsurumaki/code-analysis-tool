package main

import (
	"log"

	"github.com/spf13/cobra"

	rootcmd "github.com/k-tsurumaki/code-analysis-tool/pkg/cmd"
)

func main() {
	var root *cobra.Command
	root = rootcmd.NewRootCmd()
	if err := root.Execute(); err != nil {
		log.Fatal(err)
	}
}
