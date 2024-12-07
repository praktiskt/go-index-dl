package dl

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"

	"praktiskt/go-index-dl/utils"

	"github.com/ncruces/go-strftime"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

type DownloadStatus string

const (
	DownloadStatusFailed    = "failed"
	DownloadStatusRetry     = "retry"
	DownloadStatusCompleted = "completed"
	DownloadStatusPending   = "pending"
	DownloadStatusSkipped   = "skipped"
)

type DownloadRequest struct {
	// CreatedTimestamp represents the time when the request was created.
	CreatedTimestamp time.Time

	// FinishedTimestamp represents the time when the request was finished, or failed.
	FinishedTimestamp time.Time

	// Module represents one entry at https://index.golang.org/index?limit=1
	Module Module

	// Required defines whether the module can be skipped or not
	Required bool

	// Defines the number of retries the request is allowed to do
	Retries int
}

func NewDownloadRequest(mod Module, required bool, retries int) DownloadRequest {
	return DownloadRequest{
		CreatedTimestamp: time.Now(),
		Module:           mod,
		Required:         required,
		Retries:          retries,
	}
}

type DownloadClient struct {
	outputDir                string
	tempDir                  string
	maxTsDir                 string
	incomingDownloadRequests chan DownloadRequest
	inflightModules          utils.ConcurrentSet[string]
	completedModules         utils.ConcurrentSet[string]
	numConcurrentProcessors  int
	skipPseudoVersions       bool
	skipMaxTsWrite           bool
	stats                    stats
	numRetries               int
	currentBatch             *Modules
}

type stats struct {
	inflightRequests  utils.ConcurrentCounter[int]
	skippedRequests   utils.ConcurrentCounter[int]
	failedRequests    utils.ConcurrentCounter[int]
	completedRequests utils.ConcurrentCounter[int]
	retriedRequests   utils.ConcurrentCounter[int]
}

func newStats() stats {
	return stats{
		inflightRequests:  utils.NewConcurrentCounter[int](),
		skippedRequests:   utils.NewConcurrentCounter[int](),
		failedRequests:    utils.NewConcurrentCounter[int](),
		completedRequests: utils.NewConcurrentCounter[int](),
		retriedRequests:   utils.NewConcurrentCounter[int](),
	}
}

func (s *stats) Reset() {
	s.failedRequests.Reset()
	s.retriedRequests.Reset()
	s.skippedRequests.Reset()
	s.completedRequests.Reset()
}

func NewDownloadClient() *DownloadClient {
	return &DownloadClient{
		incomingDownloadRequests: make(chan DownloadRequest, 1),
		outputDir:                OUTPUT_DIR,
		tempDir:                  path.Join(OUTPUT_DIR, "tmp"),
		maxTsDir:                 path.Join(OUTPUT_DIR, "MAX_TS"),
		numConcurrentProcessors:  1,
		skipPseudoVersions:       false,
		skipMaxTsWrite:           false,
		completedModules:         utils.NewConcurrentSet[string](),
		inflightModules:          utils.NewConcurrentSet[string](),
		numRetries:               10,
		stats:                    newStats(),
	}
}

func (c *DownloadClient) WithOutputDir(dir string) *DownloadClient {
	c.outputDir = dir
	c.maxTsDir = path.Join(c.outputDir, "MAX_TS")
	return c
}

func (c *DownloadClient) WithTempDir(dir string) *DownloadClient {
	c.tempDir = dir
	return c
}

func (c *DownloadClient) WithNumConcurrentProcessors(cnt int) *DownloadClient {
	c.numConcurrentProcessors = cnt
	return c
}

func (c *DownloadClient) WithRequestCapacity(cnt int) *DownloadClient {
	c.incomingDownloadRequests = make(chan DownloadRequest, cnt)
	return c
}

func (c *DownloadClient) WithSkipPseudoVersions(setting bool) *DownloadClient {
	c.skipPseudoVersions = setting
	return c
}

func (c *DownloadClient) WithSkipMaxTsWrite(setting bool) *DownloadClient {
	c.skipMaxTsWrite = setting
	return c
}

func (c *DownloadClient) WithPerModuleRetries(setting int) *DownloadClient {
	c.numRetries = setting
	return c
}

func (c *DownloadClient) EnqueueBatch(mods Modules) {
	if err := createDirIfNotExist(c.tempDir); err != nil {
		slog.Error(err.Error())
	}
	c.currentBatch = &mods

	for _, mod := range mods {
		c.enqueueMod(mod, false)
	}
}

func (c *DownloadClient) enqueueMod(mod Module, required bool) {
	c.incomingDownloadRequests <- NewDownloadRequest(mod, required, c.numRetries)
}

func (c *DownloadClient) setInflight(req DownloadRequest) {
	c.inflightModules.Set(req.Module.String())
	c.stats.inflightRequests.Increment()
}

func (c *DownloadClient) completeInflight(req DownloadRequest, status DownloadStatus) {
	switch status {
	case DownloadStatusCompleted:
		c.stats.completedRequests.Increment()
	case DownloadStatusFailed:
		c.stats.failedRequests.Increment()
	case DownloadStatusSkipped:
		c.stats.skippedRequests.Increment()
	case DownloadStatusRetry:
		c.stats.retriedRequests.Increment()
	default:
		slog.Error("unmapped state", "requestStatus", status)
	}
	c.inflightModules.Delete(req.Module.String())
	c.stats.inflightRequests.Decrement()
}

// ProcessIncomingDownloadRequests blocks the thread and processes incoming DownloadRequests
func (c *DownloadClient) ProcessIncomingDownloadRequests() {
	for range c.numConcurrentProcessors {
		go func() {
			for req := range c.incomingDownloadRequests {
				if c.completedModules.Exists(req.Module.String()) || c.inflightModules.Exists(req.Module.String()) {
					continue
				}

				c.setInflight(req)
				if !req.Required && c.skipPseudoVersions && req.Module.IsPseudoVersion() {
					c.completeInflight(req, DownloadStatusSkipped)
					continue
				}
				if err := c.Download(req); err != nil {
					if req.Retries > 0 {
						req.Retries -= 1
						go func() { c.incomingDownloadRequests <- req }()
						c.completeInflight(req, DownloadStatusRetry)
						continue
					}
					slog.Error("download processor:", "modPath", req.Module.Path, "modVersion", req.Module.Version, "err", err)
					c.completeInflight(req, DownloadStatusFailed)
					continue
				}

				c.completedModules.Set(req.Module.String())
				c.completeInflight(req, DownloadStatusCompleted)
			}
		}()
	}
	<-make(chan struct{})
}

func (c *DownloadClient) AwaitInflight() {
	msg := func(m string) {
		slog.Info(m,
			"queued", len(c.incomingDownloadRequests),
			"inflight", c.stats.inflightRequests.Value(),
			"skipped", c.stats.skippedRequests.Value(),
			"retried", c.stats.retriedRequests.Value(),
			"failed", c.stats.failedRequests.Value(),
			"completed", c.stats.completedRequests.Value(),
		)
	}
	for len(c.incomingDownloadRequests) != 0 || c.stats.inflightRequests.Value() != 0 {
		msg("awaitInflight")
		time.Sleep(time.Duration(1) * time.Second)
	}
	msg("done")
	c.stats.Reset()

	if !c.skipMaxTsWrite {
		if c.currentBatch == nil {
			slog.Error("failed to update MAX_TS, no currentBatch to get timestamp from")
		}
		maxTs := c.currentBatch.GetMaxTs()
		maxTsFile := path.Join(c.maxTsDir)
		ts := strftime.Format("%Y-%m-%dT%H:%M:%S.%fZ", maxTs)
		if err := os.WriteFile(maxTsFile, []byte(ts), 0o644); err != nil {
			slog.Error("failed to write minTs to file MAX_TS:", "err", err)
		}
	}
}

// Cleanup cleans up in-flight artifacts and/or downloads.
func (c *DownloadClient) Cleanup() {
	os.RemoveAll(c.tempDir)
}

func (c *DownloadClient) Download(req DownloadRequest) error {
	if !semver.IsValid(req.Module.Version) {
		return fmt.Errorf("invalid version: %#v", req.Module)
	}

	modulePath := strings.ReplaceAll(req.Module.Path, "/", "/") // TODO: Should we really sanitize? :)
	cacheDir := path.Join(c.outputDir, modulePath, "@v")
	if err := createDirIfNotExist(cacheDir); err != nil {
		return err
	}

	// get list file
	listURL := fmt.Sprintf("%s/%s/@v/list", GO_PROXY, req.Module.Path)
	listPath := path.Join(cacheDir, "list")
	slog.Debug("downloading", "url", listURL, "targetDir", listPath)
	err := downloadFile(listPath, listURL, c.tempDir, false)
	if err != nil {
		if strings.Contains(err.Error(), `invalid escaped module path`) {
			return nil
		}
		return fmt.Errorf("failed to download list: %v", err)
	}

	modURL := req.Module.BaseURL() + ".mod"
	modPath := path.Join(cacheDir, req.Module.Version+".mod")
	slog.Debug("downloading", "url", modURL, "targetDir", modPath)
	err = downloadFile(modPath, modURL, c.tempDir, true)
	if err != nil {
		if strings.Contains(err.Error(), `invalid escaped module path`) {
			return nil
		}
		return fmt.Errorf("failed to download mod: %v", err)
	}

	modFile, err := os.Open(modPath)
	if err != nil {
		return err
	}
	defer modFile.Close()

	modData, err := io.ReadAll(modFile)
	if err != nil {
		return err
	}

	mod, err := modfile.Parse("go.mod", modData, nil)
	if err != nil {
		return err
	}

	go func(mod *modfile.File) {
		for _, req := range mod.Require {
			newMod := Module{Path: req.Mod.Path, Version: req.Mod.Version}
			c.enqueueMod(newMod, true)
		}
	}(mod)

	// get base files
	files := []string{".info", ".zip"}
	for _, ext := range files {
		fileURL := req.Module.BaseURL() + ext
		filePath := path.Join(cacheDir, req.Module.Version+ext)
		slog.Debug("downloading", "url", fileURL, "targetDir", filePath)
		if err := downloadFile(filePath, fileURL, c.tempDir, true); err != nil {
			return fmt.Errorf("failed to download %s: %v", fileURL, err)
		}
	}

	// get latest file
	latestURL := fmt.Sprintf("%s/%s/@latest", GO_PROXY, req.Module.Path)
	latestPath := path.Join(cacheDir, "latest")
	slog.Debug("downloading", "url", latestURL, "targetDir", latestPath)
	if err := downloadFile(latestPath, latestURL, c.tempDir, false); err != nil {
		return fmt.Errorf("failed to download latest: %v", err)
	}

	return nil
}
