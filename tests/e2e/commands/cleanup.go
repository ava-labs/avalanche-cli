// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/onsi/gomega"
)

func DeleteE2EInventory() {
	usr, err := user.Current()
	gomega.Expect(err).Should(gomega.BeNil())
	homeDir := usr.HomeDir
	inventoryE2E := filepath.Join(homeDir, constants.E2EBaseDirName, "nodes/inventories/", constants.E2EClusterName)
	fmt.Println("deleting: ", inventoryE2E)
	err = os.RemoveAll(inventoryE2E)
	gomega.Expect(err).Should(gomega.BeNil())
}

func DeleteNode(nodeID string) {
	if nodeID == "" {
		return
	}
	usr, err := user.Current()
	gomega.Expect(err).Should(gomega.BeNil())
	homeDir := usr.HomeDir
	nodeE2E := filepath.Join(homeDir, constants.E2EBaseDirName, "nodes", nodeID)
	fmt.Println("deleting: ", nodeE2E)
	err = os.RemoveAll(nodeE2E)
	gomega.Expect(err).Should(gomega.BeNil())
}

func DeleteE2ECluster() {
	usr, err := user.Current()
	gomega.Expect(err).Should(gomega.BeNil())
	homeDir := usr.HomeDir
	relativePath := "nodes"
	content, err := os.ReadFile(filepath.Join(homeDir, constants.E2EBaseDirName, relativePath, constants.ClustersConfigFileName))
	if err == os.ErrNotExist {
		return
	}
	gomega.Expect(err).Should(gomega.BeNil())
	clustersConfig := models.ClustersConfig{}
	err = json.Unmarshal(content, &clustersConfig)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(clustersConfig.Clusters[constants.E2EClusterName]).To(gomega.Not(gomega.BeNil()))
	clustersConfig.Clusters[constants.E2EClusterName] = models.ClusterConfig{}
	content, err = json.MarshalIndent(clustersConfig, "", "    ")
	gomega.Expect(err).Should(gomega.BeNil())
	err = os.WriteFile(filepath.Join(homeDir, constants.E2EBaseDirName, relativePath, constants.ClustersConfigFileName), content, 0o600)
	gomega.Expect(err).Should(gomega.BeNil())
}
