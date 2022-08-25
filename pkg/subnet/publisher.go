// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type Publisher interface {
	Publish(repoURL string, r *git.Repository, subnetName, vmName string, subnetYAML []byte, vmYAML []byte) error
	AddRepo(url string) (*git.Repository, error)
}

type publisherImpl struct {
	repoDir string
	repos   map[string]*url.URL
}

var _ Publisher = &publisherImpl{}

func NewPublisher(repoDir string) Publisher {
	return &publisherImpl{
		repos:   make(map[string]*url.URL),
		repoDir: repoDir,
	}
}

// TODO: this approach needs loading existing repos from disk
func (p *publisherImpl) AddRepo(repoURL string) (*git.Repository, error) {
	name := GetRepoNameFromURL(repoURL)
	u, err := url.Parse(repoURL)
	if err != nil {
		return nil, err
	}
	p.repos[name] = u
	repoPath := filepath.Join(p.repoDir, name)
	repo, err := git.PlainClone(repoPath, false, &git.CloneOptions{
		URL:      repoURL,
		Progress: os.Stdout,
	})
	if err != nil {
		return nil, err
	}
	fmt.Println("added")
	return repo, nil
}

func (p *publisherImpl) Publish(
	repoURL string,
	repo *git.Repository,
	subnetName, vmName string,
	subnetYAML []byte,
	vmYAML []byte,
) error {
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	// TODO: There needs to be a better way to get to the repo path?
	name := GetRepoNameFromURL(repoURL)
	repoPath := filepath.Join(p.repoDir, name)
	// TODO: This might not always be the right path!
	// TODO: Use constants
	subnetPath := filepath.Join(repoPath, "subnets", subnetName)
	vmPath := filepath.Join(repoPath, "vms", vmName)
	err = os.WriteFile(subnetPath, subnetYAML, 0o644) //nolint:gosec
	if err != nil {
		return err
	}

	err = os.WriteFile(vmPath, vmYAML, 0o644) //nolint:gosec
	if err != nil {
		return err
	}

	_, err = wt.Add("subnets")
	if err != nil {
		return err
	}

	_, err = wt.Add("vms")
	if err != nil {
		return err
	}

	now := time.Now()
	commitStr := fmt.Sprintf("avalanche-commit-%s", now.String())
	// TODO review these options
	commit, err := wt.Commit(commitStr, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Avalanche-CLI",
			Email: "info@avax.network", // this obviously would have to change
			When:  now,
		},
	})
	if err != nil {
		return err
	}

	_, err = repo.CommitObject(commit)
	if err != nil {
		return err
	}

	return repo.Push(&git.PushOptions{})
}

// TODO: Should we just prompt for a name instead?
func GetRepoNameFromURL(url string) string {
	// TODO: this isn't probably how we'd want to store this
	return path.Base(url)
}
