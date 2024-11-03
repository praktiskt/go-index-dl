package dl

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDownloadClient(t *testing.T) {
	defer func() { os.RemoveAll("TestDownloadClient") }()

	ts, err := time.Parse(time.RFC3339, "2019-04-10T19:08:52.997264Z")
	assert.Nil(t, err)
	mod := Module{
		Path:      "golang.org/x/text",
		Version:   "v0.3.0",
		Timestamp: ts,
	}
	req := NewDownloadRequest(mod, true)

	c := NewDownloadClient()
	assert.Nil(t, c.Download(req))
}

func TestDownloadClientLargeSample(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	defer func() { os.RemoveAll("TestDownloadClientLargeSample") }()

	dl := NewDownloadClient().
		WithRequestCapacity(10).
		WithNumConcurrentProcessors(10)
	go dl.ProcessIncomingDownloadRequests()
	ind := NewIndexClient(true)
	for range 5 {
		mods, err := ind.Scrape(10)
		assert.Nil(t, err)
		dl.EnqueueBatch(mods)
		dl.AwaitInflight()
	}
}
