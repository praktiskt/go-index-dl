package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"praktiskt/go-index-dl/dl"
	"reflect"

	"github.com/spf13/cobra"
)

var listModulesCmdConfig = struct {
	limit int
}{}

var listModulesCmd = &cobra.Command{
	Use:   "modules",
	Short: "List modules on index.golang.org",
	Run: func(cmd *cobra.Command, args []string) {
		totalMods := 0
		prevMods := dl.Modules{}
		scraper := dl.NewIndexClient(false)
		for totalMods <= listModulesCmdConfig.limit {
			mods, err := scraper.Scrape(2000)
			scraper.WithExplicitMaxTs(mods.GetMaxTs())
			if err != nil {
				slog.Error("failed to scrape", "err", err)
				os.Exit(1)
			}

			if len(mods) == 0 || reflect.DeepEqual(mods, prevMods) {
				return
			}
			prevMods = mods

			for _, mod := range mods {
				fmt.Println(mod.AsJSON())
				totalMods += 1
				if totalMods >= listModulesCmdConfig.limit {
					return
				}
			}
		}
	},
}

func init() {
	listCmd.AddCommand(listModulesCmd)
	listModulesCmd.Flags().IntVar(&listModulesCmdConfig.limit, "limit", 100, "limit the number of modules listed")
}
