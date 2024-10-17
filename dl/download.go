package dl

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"praktiskt/go-index-dl/utils"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ncruces/go-strftime"
	"golang.org/x/mod/semver"
)

type DownloadStatus string

const (
	DownloadStatusFailed    = "failed"
	DownloadStatusPending   = "pending"
	DownloadStatusProcessed = "processed"
)

type DownloadRequest struct {
	// CreatedTimestamp represents the time when the request was created.
	CreatedTimestamp time.Time

	// FinishedTimestamp represents the time when the request was finished, or failed.
	FinishedTimestamp time.Time

	// Status represents the current status of the DownloadRequest
	Status DownloadStatus

	// Module represents one entry at https://index.golang.org/index?limit=1
	Module Module
}

func NewDownloadRequest(mod Module) DownloadRequest {
	return DownloadRequest{
		CreatedTimestamp: time.Now(),
		Status:           DownloadStatusPending,
		Module:           mod,
	}
}

type DownloadClient struct {
	outputDir                string
	maxTsDir                 string
	incomingDownloadRequests chan DownloadRequest
	inflightDownloadRequests chan DownloadRequest
	skippedRequests          utils.ConcurrentCounter[int]
	failedRequests           utils.ConcurrentCounter[int]
	completedRequests        utils.ConcurrentCounter[int]
	numConcurrentProcessors  int
	skipIfNoListFile         bool
	skipPseudoVersions       bool
}

func NewDownloadClient() *DownloadClient {
	return &DownloadClient{
		incomingDownloadRequests: make(chan DownloadRequest, 1),
		inflightDownloadRequests: make(chan DownloadRequest, 1),
		outputDir:                OUTPUT_DIR,
		maxTsDir:                 path.Join(OUTPUT_DIR, "MAX_TS"),
		numConcurrentProcessors:  1,
		skipIfNoListFile:         false,
		skipPseudoVersions:       false,
		skippedRequests:          utils.NewConcurrentCounter[int](),
		failedRequests:           utils.NewConcurrentCounter[int](),
		completedRequests:        utils.NewConcurrentCounter[int](),
	}
}

func (c *DownloadClient) WithOutputDir(dir string) *DownloadClient {
	c.outputDir = dir
	c.maxTsDir = path.Join(c.outputDir, "MAX_TS")
	return c
}

func (c *DownloadClient) WithNumConcurrentProcessors(cnt int) *DownloadClient {
	c.numConcurrentProcessors = cnt
	return c
}

func (c *DownloadClient) WithRequestCapacity(cnt int) *DownloadClient {
	c.incomingDownloadRequests = make(chan DownloadRequest, cnt)
	c.inflightDownloadRequests = make(chan DownloadRequest, cnt)
	return c
}

func (c *DownloadClient) WithSkipIfNoListFile(setting bool) *DownloadClient {
	c.skipIfNoListFile = setting
	return c
}

func (c *DownloadClient) WithSkipPseudoVersions(setting bool) *DownloadClient {
	c.skipPseudoVersions = setting
	return c
}

func (c *DownloadClient) EnqueueBatch(mods []Module) {
	maxTs := time.Unix(0, 0)
	for _, mod := range mods {
		if mod.Timestamp.Unix() > maxTs.Unix() {
			maxTs = mod.Timestamp
		}
		c.enqueueMod(mod)
	}

	maxTsFile := path.Join(c.maxTsDir)
	ts := strftime.Format("%Y-%m-%dT%H:%M:%S.%fZ", maxTs)
	if err := os.WriteFile(maxTsFile, []byte(ts), 0644); err != nil {
		slog.Error("failed to write minTs to file MAX_TS:", "err", err)
	}
}

func (c *DownloadClient) enqueueMod(mod Module) {
	c.incomingDownloadRequests <- NewDownloadRequest(mod)
	c.failedRequests.Reset()
	c.skippedRequests.Reset()
	c.completedRequests.Reset()
}

// ProcessIncomingDownloadRequests blocks the thread and processes incoming DownloadRequests
func (c *DownloadClient) ProcessIncomingDownloadRequests() {
	for range c.numConcurrentProcessors {
		go func() {
			for req := range c.incomingDownloadRequests {
				c.inflightDownloadRequests <- req
				if c.skipPseudoVersions && regexp.MustCompile(`v\d+\.\d+\.\d+-(\d+\.)?(\d+\.)?\d{8}\d{6}-[a-f0-9]{12}`).MatchString(req.Module.Version) {
					c.skippedRequests.Increment()
					<-c.inflightDownloadRequests
					continue
				}
				if err := c.Download(req); err != nil {
					if c.skipIfNoListFile && strings.HasPrefix(err.Error(), "list file missing for") {
						c.skippedRequests.Increment()
						<-c.inflightDownloadRequests
						continue
					}
					slog.Error("download processor:", "err", err)
					c.failedRequests.Increment()
					<-c.inflightDownloadRequests
					continue
				}

				c.completedRequests.Increment()
				<-c.inflightDownloadRequests
			}
		}()
	}
	<-make(chan struct{})
}

func (c *DownloadClient) AwaitInflight() {
	for len(c.incomingDownloadRequests) != 0 || len(c.inflightDownloadRequests) != 0 {
		slog.Info("awaitInflight:",
			"queued", len(c.incomingDownloadRequests),
			"inflight", len(c.inflightDownloadRequests),
			"skipped", c.skippedRequests.Value(),
			"failed", c.failedRequests.Value(),
			"completed", c.completedRequests.Value(),
		)
		time.Sleep(time.Duration(1) * time.Second)
	}
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
	if c.skipIfNoListFile {
		resp, err := http.Head(listURL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		cl := resp.Header.Get("Content-Length")
		v, err := strconv.Atoi(cl)
		if err != nil {
			return err
		}
		if v == 0 {
			return fmt.Errorf("list file missing for %v", req.Module.Path)
		}
	}
	listPath := path.Join(cacheDir, "list")
	slog.Debug("downloading", "url", listURL, "targetDir", listPath)
	err := downloadFile(listPath, listURL)
	if err != nil {
		if strings.Contains(err.Error(), `invalid escaped module path`) {
			return nil
		}
		return fmt.Errorf("failed to download list: %v", err)
	}

	// get base files
	files := []string{".info", ".mod", ".zip"}
	for _, ext := range files {
		fileURL := req.Module.BaseURL() + ext
		filePath := path.Join(cacheDir, req.Module.Version+ext)
		slog.Debug("downloading", "url", fileURL, "targetDir", filePath)
		if err := downloadFile(filePath, fileURL); err != nil {
			return fmt.Errorf("failed to download %s: %v", fileURL, err)
		}
	}

	// get latest file
	latestURL := fmt.Sprintf("%s/%s/@latest", GO_PROXY, req.Module.Path)
	latestPath := path.Join(cacheDir, "latest")
	slog.Debug("downloading", "url", latestURL, "targetDir", latestPath)
	if err := downloadFile(latestPath, latestURL); err != nil {
		return fmt.Errorf("failed to download latest: %v", err)
	}

	return nil
}
