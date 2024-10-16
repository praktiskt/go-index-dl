package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	GIT_COMMIT_SHA = ""
	BUILD_TIME     = ""
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version details",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("GIT_COMMIT_SHA:", GIT_COMMIT_SHA)
		fmt.Println("BUILD_TIME:", BUILD_TIME)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// versionCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// versionCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
