// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package root

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	avalanchegoVersion = "v1.10.18"
	network            = "fuji"
	networkCapitalized = "Fuji"
	numNodes           = 1
)

var (
	hostName string
	NodeID   string
)

var _ = ginkgo.Describe("[Node monitoring]", func() {
	ginkgo.It("can create a node", func() {
		output := commands.NodeCreateWithMonitoring(network, avalanchegoVersion, numNodes)
		fmt.Println(output)
		gomega.Expect(output).To(gomega.ContainSubstring("AvalancheGo and Avalanche-CLI installed and node(s) are bootstrapping!"))
		// parse hostName
		re := regexp.MustCompile(`Generated staking keys for host (\S+)\[NodeID-(\S+)\]`)
		match := re.FindStringSubmatch(output)
		if len(match) >= 3 {
			hostName = match[1]
			NodeID = fmt.Sprintf("NodeID-%s", match[2])
		} else {
			ginkgo.Fail("failed to parse hostName and NodeID")
		}
	})
	ginkgo.It("creates cluster config", func() {
		usr, err := user.Current()
		gomega.Expect(err).Should(gomega.BeNil())
		homeDir := usr.HomeDir
		relativePath := "nodes"
		content, err := os.ReadFile(filepath.Join(homeDir, constants.BaseDirName, relativePath, constants.ClustersConfigFileName))
		gomega.Expect(err).Should(gomega.BeNil())
		clustersConfig := models.ClustersConfig{}
		err = json.Unmarshal(content, &clustersConfig)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(clustersConfig.Clusters).To(gomega.HaveLen(1))
		gomega.Expect(clustersConfig.Clusters[constants.E2EClusterName].Network.Kind.String()).To(gomega.Equal(networkCapitalized))
		gomega.Expect(clustersConfig.Clusters[constants.E2EClusterName].Nodes).To(gomega.HaveLen(numNodes))
	})
	ginkgo.It("creates node config", func() {
		fmt.Println("HostName: ", hostName)
		usr, err := user.Current()
		gomega.Expect(err).Should(gomega.BeNil())
		homeDir := usr.HomeDir
		relativePath := "nodes"
		content, err := os.ReadFile(filepath.Join(homeDir, constants.BaseDirName, relativePath, hostName, "node_cloud_config.json"))
		gomega.Expect(err).Should(gomega.BeNil())
		nodeCloudConfig := models.NodeConfig{}
		err = json.Unmarshal(content, &nodeCloudConfig)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(nodeCloudConfig.NodeID).To(gomega.Equal(hostName))
		gomega.Expect(nodeCloudConfig.ElasticIP).To(gomega.ContainSubstring(constants.E2ENetworkPrefix))
		gomega.Expect(nodeCloudConfig.CertPath).To(gomega.ContainSubstring(homeDir))
		gomega.Expect(nodeCloudConfig.UseStaticIP).To(gomega.Equal(false))
	})
})
