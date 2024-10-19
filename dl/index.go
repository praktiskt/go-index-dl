package dl

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/ncruces/go-strftime"
)

type IndexClient struct {
	BaseUrl          string
	useMaxTsFromFile bool
	maxTsLocation    string
	MaxTs            time.Time
}

func NewIndexClient(useMaxTsFromFile bool) *IndexClient {
	return &IndexClient{
		BaseUrl:          GO_INDEX,
		useMaxTsFromFile: useMaxTsFromFile,
		maxTsLocation:    path.Join(OUTPUT_DIR, "MAX_TS"),
	}
}

func (c *IndexClient) WithExplicitMaxTs(ts time.Time) {
	c.MaxTs = ts
}

func (c *IndexClient) WithMaxTsLocation(location string) *IndexClient {
	c.maxTsLocation = location
	return c
}

func (c *IndexClient) LoadMaxTsFile() error {
	maxTs, err := loadMaxTsFromFile(c.maxTsLocation)
	if err != nil {
		return err
	}
	c.MaxTs = maxTs
	return nil
}

func (c IndexClient) Scrape(limit int) (Modules, error) {
	if c.useMaxTsFromFile {
		if err := c.LoadMaxTsFile(); err != nil {
			slog.Error("failed to load MAX_TS, using default 1970-01-01", "err", err)
		}
	}
	ts := strftime.Format("%Y-%m-%dT%H:%M:%S.%fZ", c.MaxTs)
	endpoint := fmt.Sprintf("%s/index?since=%s&limit=%v", c.BaseUrl, ts, limit)
	slog.Debug("scraper", "endpoint", endpoint)

	resp, err := http.Get(endpoint)
	if err != nil {
		return []Module{}, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return []Module{}, err
	}

	if resp.StatusCode != 200 {
		return Modules{}, fmt.Errorf("server responded with %v: %v", resp.Status, string(b))
	}

	slog.Debug("scraper", "collectedBytes", len(b))

	// incoming data is jsonlines, process one by one
	modules := []Module{}
	for _, modStr := range strings.Split(string(b), "\n") {
		if len(modStr) == 0 {
			continue
		}
		mod := Module{}
		if err := json.Unmarshal([]byte(modStr), &mod); err != nil {
			slog.Error("scraper", "err", err, "modStr", modStr)
			continue
		}
		modules = append(modules, mod)
	}
	slog.Debug("scraper", "modulesScraped", len(modules))
	return modules, nil
}

func (c IndexClient) GetLatestVersion(modName string) (Module, error) {
	endpoint := fmt.Sprintf("%s/%s/@latest", GO_PROXY, modName)
	slog.Debug("GetLatestVersion", "endpoint", endpoint)
	resp, err := http.Get(endpoint)
	if err != nil {
		return Module{}, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return Module{}, err
	}
	partialMod := struct {
		Version string `json:"Version"`
		Time    string `json:"Time"`
	}{}
	if err := json.Unmarshal(b, &partialMod); err != nil {
		return Module{}, err
	}

	ts, err := time.Parse(time.RFC3339, partialMod.Time)
	if err != nil {
		return Module{}, err
	}

	return Module{
		Path:      modName,
		Version:   partialMod.Version,
		Timestamp: ts,
	}, nil
}
