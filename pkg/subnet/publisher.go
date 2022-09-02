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
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type Publisher interface {
	Publish(r *git.Repository, subnetName, vmName string, subnetYAML []byte, vmYAML []byte) error
	GetRepo() (*git.Repository, error)
}

type publisherImpl struct {
	alias    string
	repoURL  string
	repoPath string
}

var _ Publisher = &publisherImpl{}

func NewPublisher(repoDir, repoURL, alias string) Publisher {
	repoPath := filepath.Join(repoDir, alias)
	return &publisherImpl{
		alias:    alias,
		repoURL:  repoURL,
		repoPath: repoPath,
	}
}

func (p *publisherImpl) GetRepo() (repo *git.Repository, err error) {
	// path exists
	if _, err := os.Stat(p.repoPath); err == nil {
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
	subnetPath := filepath.Join(p.repoPath, constants.SubnetDir, subnetName+constants.YAMLSuffix)
	if err := os.MkdirAll(filepath.Dir(subnetPath), constants.DefaultPerms755); err != nil {
		return err
	}
	vmPath := filepath.Join(p.repoPath, constants.VMDir, vmName+constants.YAMLSuffix)
	if err := os.MkdirAll(filepath.Dir(vmPath), constants.DefaultPerms755); err != nil {
		return err
	}
	if err := os.WriteFile(subnetPath, subnetYAML, constants.DefaultPerms755); err != nil {
		return err
	}

	if err := os.WriteFile(vmPath, vmYAML, constants.DefaultPerms755); err != nil {
		return err
	}

	ux.Logger.PrintToUser("Adding resources to local git repo...")

	if _, err := wt.Add("subnets"); err != nil {
		return err
	}

	if _, err := wt.Add("vms"); err != nil {
		return err
	}

	ux.Logger.PrintToUser("Committing resources to local git repo...")
	now := time.Now()
	commitStr := fmt.Sprintf("avalanche-commit-%s", now.String())

	// use the global git config to try identifying the author
	conf, err := config.LoadConfig(config.GlobalScope)
	authorName := conf.Author.Name
	authorEmail := conf.Author.Email
	if err != nil || authorName == "" || authorEmail == "" { // a commit must have both
		authorName = constants.GitRepoCommitName
		authorEmail = constants.GitRepoCommitEmail
	}

	commit, err := wt.Commit(commitStr, &git.CommitOptions{
		Author: &object.Signature{
			Name:  authorName,
			Email: authorEmail,
			When:  now,
		},
	})
	if err != nil {
		return err
	}

	if _, err := repo.CommitObject(commit); err != nil {
		return err
	}

	ux.Logger.PrintToUser("Pushing to remote...")
	return repo.Push(&git.PushOptions{})
}
