package dl

import (
	"fmt"
	"log/slog"
	"os"
	"path"
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
	incomingDownloadRequests chan DownloadRequest
	inflightDownloadRequests chan DownloadRequest
	numConcurrentProcessors  int
}

func NewDownloadClient(incomingRequestCapacity int, numConcurrentProcessors int) DownloadClient {
	return DownloadClient{
		incomingDownloadRequests: make(chan DownloadRequest, incomingRequestCapacity),
		inflightDownloadRequests: make(chan DownloadRequest, incomingRequestCapacity),
		outputDir:                OUTPUT_DIR,
		numConcurrentProcessors:  numConcurrentProcessors,
	}
}

func (c DownloadClient) EnqueueBatch(mods []Module) {
	maxTs := time.Unix(0, 0)
	for _, mod := range mods {
		if mod.Timestamp.Unix() > maxTs.Unix() {
			maxTs = mod.Timestamp
		}
		c.enqueueMod(mod)
	}

	minTsFile := path.Join(MAX_TS_FILE)
	ts := strftime.Format("%Y-%m-%dT%H:%M:%S.%fZ", maxTs)
	if err := os.WriteFile(minTsFile, []byte(ts), 0644); err != nil {
		slog.Error("failed to write minTs to file MAX_TS:", "err", err)
	}
}

func (c DownloadClient) enqueueMod(mod Module) {
	c.incomingDownloadRequests <- NewDownloadRequest(mod)
}

// ProcessIncomingDownloadRequests blocks the thread and processes incoming DownloadRequests
func (c DownloadClient) ProcessIncomingDownloadRequests() {
	for range c.numConcurrentProcessors {
		go func() {
			for req := range c.incomingDownloadRequests {
				c.inflightDownloadRequests <- req
				if err := c.Download(req); err != nil {
					slog.Error("download processor:", "err", err)
				}
				<-c.inflightDownloadRequests
			}
		}()
	}
	<-make(chan struct{})
}

func (c DownloadClient) AwaitInflight() {
	for len(c.incomingDownloadRequests) != 0 || len(c.inflightDownloadRequests) != 0 {
		slog.Info("awaitInflight:", "queued", len(c.incomingDownloadRequests), "inflight", len(c.inflightDownloadRequests))
		time.Sleep(time.Duration(1) * time.Second)
	}
}

func (c DownloadClient) Download(req DownloadRequest) error {
	if !semver.IsValid(req.Module.Version) {
		return fmt.Errorf("invalid version: %#v", req.Module)
	}

	modulePath := strings.ReplaceAll(req.Module.Path, "/", "/") // TODO: Should we really sanitize? :)
	cacheDir := path.Join(c.outputDir, modulePath, "@v")
	if err := createDirIfNotExist(cacheDir); err != nil {
		return err
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

	// get list file
	listURL := fmt.Sprintf("%s/%s/@v/list", GO_PROXY, req.Module.Path)
	listPath := path.Join(cacheDir, "list")
	slog.Debug("downloading", "url", listURL, "targetDir", listPath)
	if err := downloadFile(listPath, listURL); err != nil {
		return fmt.Errorf("failed to download list: %v", err)
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
