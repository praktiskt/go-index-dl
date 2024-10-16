package cmd

import (
	"log/slog"
	"praktiskt/go-index-dl/dl"

	"github.com/spf13/cobra"
)

var syncModulesCmdConfig = struct {
	concurrentProcessors int
	batchSize            int
}{}

var syncModulesCmd = &cobra.Command{
	Use:   "modules",
	Short: "Sync modules from index.golang.org to a local directory",
	Run: func(cmd *cobra.Command, args []string) {
		if syncModulesCmdConfig.batchSize <= 1 || syncModulesCmdConfig.batchSize > 2000 {
			slog.Error("batch-size must be between 2 and 2000 inclusive")
		}
		dlc := dl.NewDownloadClient(
			syncModulesCmdConfig.batchSize,
			syncModulesCmdConfig.concurrentProcessors,
		)
		go dlc.ProcessIncomingDownloadRequests()
		ind := dl.NewIndexClient(true)
		for {
			mods, err := ind.Scrape(syncModulesCmdConfig.batchSize)
			if err != nil {
				slog.Error("failed to scrape", "err", err)
			}
			dlc.EnqueueBatch(mods)
			dlc.AwaitInflight()
			slog.Info("finished writing batch", "maxTs", mods.GetMaxTs().String())
		}
	},
}

func init() {
	syncCmd.AddCommand(syncModulesCmd)
	syncModulesCmd.Flags().IntVarP(&syncModulesCmdConfig.batchSize, "batch-size", "b", 2000, "batch these many requests at most, should a batch fail sync will restart from the last successful batch (min=2, max=2000)")
	syncModulesCmd.Flags().IntVarP(&syncModulesCmdConfig.concurrentProcessors, "concurrent-processors", "c", 10, "number of concurrent processors processing requests, reducing it will reduce network i/o")
}
