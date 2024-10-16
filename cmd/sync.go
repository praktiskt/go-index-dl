package cmd

import (
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use: "sync",
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
