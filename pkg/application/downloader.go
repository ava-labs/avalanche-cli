package application

import (
	"fmt"
	"io"
	"net/http"
)

type Downloader interface {
	Download(url string) ([]byte, error)
}

type downloader struct{}

func NewDownloader() Downloader {
	return &downloader{}
}

func (downloader) Download(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected http status code: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
