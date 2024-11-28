package dl

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func GetEnvOr(env string, fallback string) string {
	if v, ok := os.LookupEnv(env); ok {
		return v
	}
	return fallback
}

func createDirIfNotExist(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, os.ModePerm)
	}
	return nil
}

func downloadFile(filepath string, url string, tempDir string, skipIfExists bool) error {
	if skipIfExists && fileExists(filepath) {
		return nil
	}

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("server responded with %v: %v", resp.Status, string(b))
	}

	tmpFile, err := os.CreateTemp(tempDir, "go-index-dl")
	if err != nil {
		return err
	}
	defer tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return err
	}

	if err := os.Chmod(tmpFile.Name(), 0644); err != nil {
		return err
	}

	if err := os.Rename(tmpFile.Name(), filepath); err != nil {
		return err
	}

	return nil
}

func loadMaxTsFromFile(maxTsDir string) (time.Time, error) {
	data, err := os.ReadFile(maxTsDir)
	if err != nil {
		return time.Time{}, err
	}
	ts, err := time.Parse(time.RFC3339, string(data))
	if err != nil {
		return time.Time{}, err
	}
	return ts, nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
