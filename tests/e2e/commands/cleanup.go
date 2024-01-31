// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package commands

import (
	"encoding/json"
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
	inventoryE2E := filepath.Join(homeDir, constants.BaseDirName, "nodes/inventories/", constants.E2EClusterName)
	err = os.RemoveAll(inventoryE2E)
	gomega.Expect(err).Should(gomega.BeNil())
}

func DeleteNode(nodeID string) {
	usr, err := user.Current()
	gomega.Expect(err).Should(gomega.BeNil())
	homeDir := usr.HomeDir
	nodeE2E := filepath.Join(homeDir, constants.BaseDirName, "nodes", nodeID)
	err = os.RemoveAll(nodeE2E)
	gomega.Expect(err).Should(gomega.BeNil())
}

func DeleteE2ECluster() {
	usr, err := user.Current()
	gomega.Expect(err).Should(gomega.BeNil())
	homeDir := usr.HomeDir
	relativePath := "nodes"
	content, err := os.ReadFile(filepath.Join(homeDir, constants.BaseDirName, relativePath, constants.ClustersConfigFileName))
	gomega.Expect(err).Should(gomega.BeNil())
	clustersConfig := models.ClustersConfig{}
	err = json.Unmarshal(content, &clustersConfig)
	gomega.Expect(err).Should(gomega.BeNil())
	gomega.Expect(clustersConfig.Clusters[constants.E2EClusterName]).To(gomega.Not(gomega.BeNil()))
	clustersConfig.Clusters[constants.E2EClusterName] = models.ClusterConfig{}
	content, err = json.Marshal(clustersConfig)
	gomega.Expect(err).Should(gomega.BeNil())
	err = os.WriteFile(filepath.Join(homeDir, constants.BaseDirName, relativePath, constants.ClustersConfigFileName), content, 0o644)
	gomega.Expect(err).Should(gomega.BeNil())
}
