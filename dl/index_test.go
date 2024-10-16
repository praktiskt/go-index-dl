package dl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScrape(t *testing.T) {
	c := NewIndexClient(false)
	modules, err := c.Scrape(10)
	assert.Nil(t, err)
	assert.Greater(t, len(modules), 0)
}
