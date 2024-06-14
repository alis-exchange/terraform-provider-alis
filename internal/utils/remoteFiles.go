package utils

import (
	"context"
	"io"
	"net/http"
)

// ReadUrl is a utility function to read an object from a remote URL
func ReadUrl(ctx context.Context, url string) ([]byte, error) {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Parse object data
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}
