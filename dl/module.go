package dl

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"
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

func (m Module) AsJSON() string {
	b, err := json.Marshal(m)
	if err != nil {
		slog.Error("failed to marshal into json", "err", err)
		os.Exit(1)
	}
	return string(b)
}

type Modules []Module

func (ms Modules) GetMaxTs() time.Time {
	maxTs := time.Unix(0, 0)
	for _, m := range ms {
		if m.Timestamp.UnixNano() >= maxTs.UnixNano() {
			maxTs = m.Timestamp
		}
	}
	return maxTs
}
