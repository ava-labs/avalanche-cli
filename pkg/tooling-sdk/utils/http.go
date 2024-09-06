// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"fmt"
	"io"
	"net/http"
)

func HTTPGet(url, authToken string) ([]byte, error) {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed downloading %s: %w", url, err)
	}
	if authToken != "" {
		// to avoid rate limitation issues
		request.Header.Set("authorization", fmt.Sprintf("Bearer %s", authToken))
	}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed downloading %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed downloading %s: unexpected http status code: %d", url, resp.StatusCode)
	}
	defer resp.Body.Close()
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed downloading %s: %w", url, err)
	}
	return bs, nil
}
