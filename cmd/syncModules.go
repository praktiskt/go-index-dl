package cmd

import (
	"log/slog"
	"praktiskt/go-index-dl/dl"

	"github.com/spf13/cobra"
)

var syncModulesCmdConfig = struct {
	requestBufferSize    int
	concurrentProcessors int
	batchSize            int
}{}

var syncModulesCmd = &cobra.Command{
	Use:   "modules",
	Short: "Sync modules from index.golang.org to a local directory",
	Run: func(cmd *cobra.Command, args []string) {
		dlc := dl.NewDownloadClient(
			syncModulesCmdConfig.requestBufferSize,
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
		}
	},
}

func init() {
	syncCmd.AddCommand(syncModulesCmd)
	syncModulesCmd.Flags().IntVar(&syncModulesCmdConfig.requestBufferSize, "request-buffer-size", 2000, "buffer at most these many requests")
	syncModulesCmd.Flags().IntVarP(&syncModulesCmdConfig.concurrentProcessors, "concurrent-processors", "c", 10, "number of concurrent processors processing requests")
	syncModulesCmd.Flags().IntVar(&syncModulesCmdConfig.batchSize, "batch-size", 2000, "batch these many requests at most")
}
