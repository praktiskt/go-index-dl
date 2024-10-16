package dl

import "fmt"

var (
	GO_PROXY    = GetEnvOr("GO_PROXY", "https://proxy.golang.org")
	GO_INDEX    = GetEnvOr("GO_INDEX", "https://index.golang.org")
	OUTPUT_DIR  = GetEnvOr("OUTPUT_DIR", "go_pkg")
	MAX_TS_FILE = GetEnvOr("MIN_TS_FILE", fmt.Sprintf("%v/MAX_TS", OUTPUT_DIR))
)
