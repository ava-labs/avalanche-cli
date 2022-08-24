// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	git "github.com/go-git/go-git/v5"
)

type Publisher interface {
	Publish(subnetYAML []byte, vmYAML []byte) error
}

type publisherImpl struct {
	url string
}

var _ Publisher = &publisherImpl{}

func NewPublisher(url string) Publisher {
	return &publisherImpl{
		url: url,
	}
}

func (p *publisherImpl) Publish(subnetYAML []byte, vmYAML []byte) error {
	repo, err := git.PlainOpen(p.url)
	if err != nil {
		return err
	}
	return repo.Push(&git.PushOptions{})
}
