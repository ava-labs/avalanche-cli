// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package binutils

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
)

type Installer interface {
	GetArch() (string, string)
	DownloadRelease(releaseURL string) ([]byte, error)
}

type installerImpl struct{}

func NewInstaller() Installer {
	return &installerImpl{}
}

func (installerImpl) GetArch() (string, string) {
	return runtime.GOARCH, runtime.GOOS
}

func (installerImpl) DownloadRelease(releaseURL string) ([]byte, error) {
	resp, err := http.Get(releaseURL)
	if err != nil {
		return []byte{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return []byte{}, fmt.Errorf("unexpected http status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	archive, err := io.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	}
	return archive, nil
}
