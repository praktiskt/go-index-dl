package cmd

import (
	"log/slog"
	"os"
	"path"
	"praktiskt/go-index-dl/dl"
	"time"

	"github.com/spf13/cobra"
)

var getModuleCmdConfig = struct {
	tempDir       string
	outputDir     string
	moduleName    string
	moduleVersion string
}{}

var getModuleCmd = &cobra.Command{
	Use:   "module",
	Short: "Get a single module from proxy.golang.org",
	Run: func(cmd *cobra.Command, args []string) {
		if getModuleCmdConfig.moduleName == "" {
			slog.Error("must provide a module name")
			os.Exit(1)
		}

		dlc := dl.NewDownloadClient().
			WithOutputDir(getModuleCmdConfig.outputDir).
			WithTempDir(getModuleCmdConfig.tempDir).
			WithSkipMaxTsWrite(true)

		go dlc.ProcessIncomingDownloadRequests()
		mod := dl.Module{Path: getModuleCmdConfig.moduleName, Version: getModuleCmdConfig.moduleVersion}
		if getModuleCmdConfig.moduleVersion == "latest" {
			modl, err := dl.NewIndexClient(false).GetLatestVersion(getModuleCmdConfig.moduleName)
			if err != nil {
				slog.Error("failed to get latest version", "err", err)
				os.Exit(1)
			}
			mod = modl
		}
		mods := dl.Modules{mod}
		dlc.EnqueueBatch(mods)
		time.Sleep(time.Duration(500) * time.Millisecond) // TODO: this solves race condition, but we can do it better
		dlc.AwaitInflight()
	},
}

func init() {
	getCmd.AddCommand(getModuleCmd)
	getModuleCmd.Flags().StringVarP(&getModuleCmdConfig.outputDir, "output-dir", "o", dl.OUTPUT_DIR, "the absolute or relative path to the output directory (can also be set with OUTPUT_DIR)")
	getModuleCmd.Flags().StringVar(&getModuleCmdConfig.tempDir, "temp-dir", path.Join(dl.OUTPUT_DIR, "tmp"), "the place to store temporary artifacts in")
	getModuleCmd.Flags().StringVarP(&getModuleCmdConfig.moduleName, "module-name", "m", "", "the name of the module to download, e.g. golang.org/x/exp")
	getModuleCmd.Flags().StringVarP(&getModuleCmdConfig.moduleVersion, "module-version", "v", "latest", "the version of the module to download, can be a semver version or 'latest'")
}
