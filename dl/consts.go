package dl

var (
	GO_PROXY   = GetEnvOr("GO_PROXY", "https://proxy.golang.org")
	GO_INDEX   = GetEnvOr("GO_INDEX", "https://index.golang.org")
	OUTPUT_DIR = GetEnvOr("OUTPUT_DIR", "go_pkg")
)
