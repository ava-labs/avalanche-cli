// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
)

var (
	client             = http.DefaultClient
	errHTTPStatusNotOK = errors.New("non-200 HTTP status code")
)

func MakeGetRequest(ctx context.Context, url string) ([]byte, error) {
	return MakeGetRequestWithAuthToken(ctx, url, "")
}

func MakeGetRequestWithAuthToken(ctx context.Context, url string, authToken string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if authToken != "" {
		req.Header.Set("authorization", "Bearer "+authToken)
	}
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errHTTPStatusNotOK
	}

	return io.ReadAll(resp.Body)
}

func ValidateURLFormat(input string) error {
	_, err := url.ParseRequestURI(input)
	return err
}
