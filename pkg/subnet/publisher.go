// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type Publisher interface {
	Publish(r *git.Repository, subnetName, vmName string, subnetYAML []byte, vmYAML []byte) error
	GetRepo() (*git.Repository, error)
}

type publisherImpl struct {
	alias    string
	repoDir  string
	repoURL  string
	repoPath string
}

var _ Publisher = &publisherImpl{}

func NewPublisher(repoDir, repoURL, alias string) Publisher {
	repoPath := filepath.Join(repoDir, alias)
	return &publisherImpl{
		alias:    alias,
		repoDir:  repoDir,
		repoURL:  repoURL,
		repoPath: repoPath,
	}
}

func (p *publisherImpl) GetRepo() (repo *git.Repository, err error) {
	// path exists
	if _, err = os.Stat(p.repoPath); err == nil {
		return git.PlainOpen(p.repoPath)
	}
	return git.PlainClone(p.repoPath, false, &git.CloneOptions{
		URL:      p.repoURL,
		Progress: os.Stdout,
	})
}

func (p *publisherImpl) Publish(
	repo *git.Repository,
	subnetName, vmName string,
	subnetYAML []byte,
	vmYAML []byte,
) error {
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	// TODO: This might not always be the right path!
	// TODO: Use constants
	subnetPath := filepath.Join(p.repoPath, "subnets", subnetName)
	vmPath := filepath.Join(p.repoPath, "vms", vmName)
	err = os.WriteFile(subnetPath, subnetYAML, constants.DefaultPerms755)
	if err != nil {
		return err
	}

	err = os.WriteFile(vmPath, vmYAML, constants.DefaultPerms755)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("Adding resources to local git repo...")

	_, err = wt.Add("subnets")
	if err != nil {
		return err
	}

	_, err = wt.Add("vms")
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("Committing resources to local git repo...")
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

	ux.Logger.PrintToUser("Pushing to remote...")
	return repo.Push(&git.PushOptions{})
}
