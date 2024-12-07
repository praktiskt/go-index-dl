package cmd

import (
	"log/slog"
	"path"
	"time"

	"praktiskt/go-index-dl/dl"

	"github.com/spf13/cobra"
)

var syncModulesCmdConfig = struct {
	concurrentProcessors int
	batchSize            int
	outputDir            string
	tempDir              string
	numRetries           int
	skipPseudoVersions   bool
	exitOnEnd            bool
}{}

var syncModulesCmd = &cobra.Command{
	Use:   "modules",
	Short: "Sync modules from index.golang.org to a local directory",
	Long: `This command will sync all modules from index.golang.org. This takes a while
since there are millions of module+versions.

The command will on each batch completion update a MAX_TS-file in the output directory,
containing max timestamp of the last successful batch. This timestamp is used to
determine where to collect modules from.`,
	Run: func(cmd *cobra.Command, args []string) {
		if syncModulesCmdConfig.batchSize <= 1 || syncModulesCmdConfig.batchSize > 2000 {
			slog.Error("batch-size must be between 2 and 2000 inclusive")
		}
		dlc := dl.NewDownloadClient().
			WithNumConcurrentProcessors(syncModulesCmdConfig.concurrentProcessors).
			WithOutputDir(syncModulesCmdConfig.outputDir).
			WithTempDir(syncModulesCmdConfig.tempDir).
			WithRequestCapacity(syncModulesCmdConfig.batchSize).
			WithSkipPseudoVersions(syncModulesCmdConfig.skipPseudoVersions)
		defer dlc.Cleanup()
		go dlc.ProcessIncomingDownloadRequests()

		ind := dl.NewIndexClient(true).
			WithMaxTsLocation(path.Join(syncModulesCmdConfig.outputDir, "MAX_TS"))

		for {
			mods, err := ind.Scrape(syncModulesCmdConfig.batchSize)
			if err != nil {
				slog.Error("failed to scrape", "err", err)
				continue
			}
			if len(mods) < syncModulesCmdConfig.batchSize {
				if syncModulesCmdConfig.exitOnEnd {
					slog.Info("the end has been reached, exiting")
					break
				}
				slog.Info("very few modules collected, sleeping for 60 seconds before trying again")
				time.Sleep(time.Duration(60) * time.Second)
				continue
			}
			dlc.EnqueueBatch(mods)
			dlc.AwaitInflight()
			dlc.Cleanup()
			slog.Info("finished writing batch", "maxTs", mods.GetMaxTs().String())
		}
	},
}

func init() {
	syncCmd.AddCommand(syncModulesCmd)
	syncModulesCmd.Flags().IntVarP(&syncModulesCmdConfig.batchSize, "batch-size", "b", 2000, "batch these many requests at most, should a batch fail sync will restart from the last successful batch (min=2, max=2000)")
	syncModulesCmd.Flags().IntVarP(&syncModulesCmdConfig.concurrentProcessors, "concurrent-processors", "c", 10, "number of concurrent processors processing requests, reducing it will reduce network i/o")
	syncModulesCmd.Flags().IntVar(&syncModulesCmdConfig.numRetries, "num-retries", 10, "number of times to retry a module download if it fails")
	syncModulesCmd.Flags().StringVarP(&syncModulesCmdConfig.outputDir, "output-dir", "o", dl.OUTPUT_DIR, "the absolute or relative path to the output directory (can also be set with OUTPUT_DIR)")
	syncModulesCmd.Flags().StringVar(&syncModulesCmdConfig.tempDir, "temp-dir", path.Join(dl.OUTPUT_DIR, "tmp"), "the place to store temporary artifacts in")
	syncModulesCmd.Flags().BoolVar(&syncModulesCmdConfig.skipPseudoVersions, "skip-pseudo-versions", true, "skip pseudo versions unless they are required by a non-pseudo-version, see https://go.dev/ref/mod#glos-pseudo-version")
	syncModulesCmd.Flags().BoolVar(&syncModulesCmdConfig.exitOnEnd, "exit-on-end", false, "when reaching 'the end' of the current listing, exit the program")
}
