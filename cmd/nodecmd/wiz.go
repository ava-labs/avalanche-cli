// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/spf13/cobra"
)

func newWizCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wiz [clusterName] [subnetName]",
		Short: "(ALPHA Warning) Creates a devnet together with a fully validated subnet into it.",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node wiz command creates a devnet and deploys, sync and validate a subnet into it. It creates the subnet if so needed.
`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE:         wiz,
	}
	cmd.Flags().BoolVar(&useStaticIP, "use-static-ip", true, "attach static Public IP on cloud servers")
	cmd.Flags().BoolVar(&useAWS, "aws", false, "create node/s in AWS cloud")
	cmd.Flags().BoolVar(&useGCP, "gcp", false, "create node/s in GCP cloud")
	cmd.Flags().StringVar(&cmdLineRegion, "region", "", "create node/s in given region")
	cmd.Flags().BoolVar(&authorizeAccess, "authorize-access", false, "authorize CLI to create cloud resources")
	cmd.Flags().IntVar(&numNodes, "num-nodes", 0, "number of nodes to create")
	cmd.Flags().StringVar(&cmdLineGCPCredentialsPath, "gcp-credentials", "", "use given GCP credentials")
	cmd.Flags().StringVar(&cmdLineGCPProjectName, "gcp-project", "", "use given GCP project")
	cmd.Flags().StringVar(&cmdLineAlternativeKeyPairName, "alternative-key-pair-name", "", "key pair name to use if default one generates conflicts")
	cmd.Flags().StringVar(&awsProfile, "aws-profile", constants.AWSDefaultCredential, "aws profile to use")
	return cmd
}

func wiz(cmd *cobra.Command, args []string) error {
	clusterName := args[0]
	subnetName := args[1]
	createDevnet = true
	useAvalanchegoVersionFromSubnet = subnetName
	// check there is no clusterName ...
	// if there is no subnet, create it
	return createNodes(cmd, []string{clusterName})
}
