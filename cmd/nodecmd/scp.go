// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

func newSCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scp SOURCE DEST",
		Short: "(ALPHA Warning) Securely copy files to and from nodes",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node scp command securely copies files to and from nodes. Remote source or destionation can be specified using the following format:
[clusterName|nodeID|instanceID|IP]:/path/to/file. Regular experssions are supported for the source files like /tmp/*.txt.
File transfer to the nodes are parallelized. IF source or destination is cluster, the other should be a local file path. 
If both destinations are remote, they must be nodes for the same cluster and not clusters themselves.
For example:
$ avalanche node scp [cluster1|node1]:/tmp/file.txt /tmp/file.txt
$ avalanche node scp /tmp/file.txt [cluster1|NodeID-XXXX]:/tmp/file.txt
$ avalanche node scp node1:/tmp/file.txt NodeID-XXXX:/tmp/file.txt
`,
		Args: cobrautils.MinimumNArgs(2),
		RunE: scpNode,
	}
	return cmd
}

func scpNode(_ *cobra.Command, args []string) error {
	var err error
	clustersConfig := models.ClustersConfig{}
	if app.ClustersConfigExists() {
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return err
		}
	}
	if len(clustersConfig.Clusters) == 0 {
		ux.Logger.PrintToUser("There are no clusters defined.")
		return nil
	}

	sourcePath, destPath := args[0], args[1]
	sourceClusterNameOrNodeID, sourcePath := utils.SplitScpPath(sourcePath)
	destClusterNameOrNodeID, destPath := utils.SplitScpPath(destPath)

	// check if source and destination are both clusters
	sourceClusterExists, err := checkClusterExists(sourceClusterNameOrNodeID)
	if err != nil {
		return err
	}
	destClusterExists, err := checkClusterExists(destClusterNameOrNodeID)
	if err != nil {
		return err
	}
	if sourceClusterExists && destClusterExists {
		return fmt.Errorf("both source and destination cannot be clusters")
	}

	if err := checkCluster(sourceClusterNameOrNodeID); err == nil {
		// source is a cluster
		clusterName := sourceClusterNameOrNodeID
		clusterHosts, err := GetAllClusterHosts(clusterName)
		if err != nil {
			return err
		}
		return scpHosts(clusterHosts, sourcePath, destPath, clusterName)
	}
	if err := checkCluster(destClusterNameOrNodeID); err == nil {
		// destination is a cluster
		clusterName := destClusterNameOrNodeID
		clusterHosts, err := GetAllClusterHosts(clusterName)
		if err != nil {
			return err
		}
		return scpHosts(clusterHosts, destPath, sourcePath, clusterName)
	}
	return nil
}

func scpHosts(hosts []*models.Host, sourcePath, destPath string, clusterName string) error {
	// get source and destination
	source, err := prepareSCPDestination(clusterName, sourcePath)
	if err != nil {
		return err
	}
	dest, err := prepareSCPDestination(clusterName, destPath)
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults models.NodeResults, host *models.Host) {
			defer wg.Done()
			nodeConf, err := app.LoadClusterNodeConfig(nodeName)
			if err != nil {
				nodeResults.AddResult(host.NodeID, "", err)
				return
			}
			scpCmd := utils.GetSCPCommandString(nodeConf.CertPath)

		}(wgResults, host)
	}

}

// prepareSCPDestination prepares the destination for scp command
func prepareSCPDestination(clusterName string, dest string) (string, error) {
	//valid clusterName - is already checked
	if !strings.Contains(dest, ":") {
		//destination is local, ready to go
		return dest, nil
	}
	//destination is remote
	splitDest := strings.Split(dest, ":")
	node := splitDest[0]
	path := splitDest[1]
	if utils.IsValidIP(node) {
		//destination is IP, ready to go
		return dest, nil
	}
	//destination is cloudID or NodeID. clusterName is already checked and valid
	clusterHosts, err := GetAllClusterHosts(clusterName)
	if err != nil {
		return "", err
	}
	selectedHost := utils.Filter(clusterHosts, func(h *models.Host) bool {
		_, cloudHostID, _ := models.HostAnsibleIDToCloudID(h.NodeID)
		hostNodeID, _ := getNodeID(app.GetNodeInstanceDirPath(cloudHostID))
		return h.GetCloudID() == node || hostNodeID.String() == node
	})
	switch {
	case len(selectedHost) == 0:
		return "", fmt.Errorf("node %s not found in cluster %s", node, clusterName)
	case len(selectedHost) > 2:
		return "", fmt.Errorf("more then 1 node found for %s in cluster %s", node, clusterName)
	default:
		return fmt.Sprintf("%s:%s", selectedHost[0].IP, path), nil
	}
}
