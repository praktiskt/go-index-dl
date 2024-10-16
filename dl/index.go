package dl

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ncruces/go-strftime"
)

// Module represents one entry at https://index.golang.org/index?limit=1
type Module struct {
	Timestamp time.Time
	Path      string
	Version   string
}

func (m Module) BaseURL() string {
	return fmt.Sprintf("%s/%s/@v/%s", GO_PROXY, m.Path, m.Version)
}

type IndexClient struct {
	BaseUrl          string
	useMinTsFromFile bool
	minTs            time.Time
}

func NewIndexClient(useMinTsFromFile bool) IndexClient {
	return IndexClient{
		BaseUrl:          GO_INDEX,
		useMinTsFromFile: useMinTsFromFile,
	}
}

func (c *IndexClient) LoadMinTsFile() error {
	minTs, err := loadMaxTsFromFile()
	if err != nil {
		return err
	}
	c.minTs = minTs
	return nil
}

func (c IndexClient) Scrape(limit int) ([]Module, error) {
	if c.useMinTsFromFile {
		if err := c.LoadMinTsFile(); err != nil {
			slog.Error("failed to load MIN_TS, using default 1970-01-01", "err", err)
		}
	}
	ts := strftime.Format("%Y-%m-%dT%H:%M:%S.%fZ", c.minTs)
	endpoint := fmt.Sprintf("%s/index?since=%s&limit=%v", c.BaseUrl, ts, limit)
	slog.Info("scraper", "endpoint", endpoint)

	resp, err := http.Get(endpoint)
	if err != nil {
		return []Module{}, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return []Module{}, err
	}
	slog.Info("scraper", "collectedBytes", len(b))

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
	slog.Info("scraper", "modulesScraped", len(modules))
	return modules, nil
}
