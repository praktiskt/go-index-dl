package dl

import (
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

func downloadFile(filepath string, url string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
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
