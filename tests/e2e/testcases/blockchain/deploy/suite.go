// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package deploy

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"

	"github.com/ava-labs/avalanche-cli/pkg/signatureaggregator"

	"github.com/ava-labs/avalanche-cli/cmd"
	"github.com/ava-labs/avalanche-cli/pkg/blockchain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanche-cli/sdk/validatormanager/validatormanagertypes"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/ethereum/go-ethereum/common"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	subnetName = "testSubnet"
)

const ewoqEVMAddress = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"

func checkConvertOnlyOutput(output string, generateNodeID bool, subnetName string) {
	gomega.Expect(output).Should(gomega.ContainSubstring("Converted blockchain successfully generated"))
	gomega.Expect(output).Should(gomega.ContainSubstring("Have the Avalanche node(s) track the blockchain"))
	gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("Call `avalanche contract initValidatorManager %s`", subnetName)))
	gomega.Expect(output).Should(gomega.ContainSubstring("Ensure that the P2P port is exposed and 'public-ip' config value is set"))
	gomega.Expect(output).ShouldNot(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
	if generateNodeID {
		gomega.Expect(output).Should(gomega.ContainSubstring("Create the corresponding Avalanche node(s) with the provided Node ID and BLS Info"))
	} else {
		gomega.Expect(output).ShouldNot(gomega.ContainSubstring("Create the corresponding Avalanche node(s) with the provided Node ID and BLS Info"))
	}
}

var _ = ginkgo.Describe("[Blockchain Deploy]", ginkgo.Ordered, func() {
	_ = ginkgo.BeforeEach(func() {
		// Create test subnet config
		commands.CreateEtnaSubnetEvmConfig(subnetName, ewoqEVMAddress, commands.PoA)
	})

	ginkgo.AfterEach(func() {
		commands.CleanNetwork()
		listSigAggCmd := exec.Command("./bin/avalanche", "interchain", "signatureAggregator", "list", "--local")
		outputBytes, err := listSigAggCmd.CombinedOutput()
		gomega.Expect(err).Should(gomega.BeNil())
		output := string(outputBytes)
		gomega.Expect(output).Should(gomega.ContainSubstring("No locally run signature aggregator found for Local Network"))

		// Cleanup test subnet config
		commands.DeleteSubnetConfig(subnetName)
	})
	blockchainCmdArgs := []string{subnetName}
	globalFlags := utils.GlobalFlags{
		"local":             true,
		"skip-icm-deploy":   true,
		"skip-update-check": true,
	}
	ginkgo.It("HAPPY PATH: local deploy default", func() {
		testFlags := utils.TestFlags{}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())
		localClusterUris, err := utils.GetLocalClusterUris()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(len(localClusterUris)).Should(gomega.Equal(1))
		// check that validator manager type is proof of authority
		sc, err := utils.GetSideCar(blockchainCmdArgs[0])
		gomega.Expect(err).Should(gomega.BeNil())
		subnetInfo, _ := blockchain.GetSubnet(sc.Networks["Local Network"].SubnetID, models.NewLocalNetwork())
		validatorManagerAddress := "0x" + hex.EncodeToString(subnetInfo.ManagerAddress)
		uri := fmt.Sprintf("%s/ext/bc/%s/rpc", localClusterUris[0], sc.Networks["Local Network"].BlockchainID)
		valType := validatorManagerSDK.GetValidatorManagerType(uri, common.HexToAddress(validatorManagerAddress))
		expectedValType := validatormanagertypes.ValidatorManagementTypeFromString(validatormanagertypes.ProofOfAuthority)
		gomega.Expect(valType).Should(gomega.Equal(expectedValType))
		listSigAggCmd := exec.Command("./bin/avalanche", "interchain", "signatureAggregator", "list", "--local")
		outputBytes, err := listSigAggCmd.CombinedOutput()
		gomega.Expect(err).Should(gomega.BeNil())
		output = string(outputBytes)
		gomega.Expect(output).ShouldNot(gomega.ContainSubstring("No locally run signature aggregator found for Local Network"))
	})

	ginkgo.It("HAPPY PATH: local deploy with avalanchego path set", func() {
		avalanchegoPath := "tests/e2e/assets/mac/avalanchego"
		if runtime.GOOS == "linux" {
			avalanchegoPath = "tests/e2e/assets/linux/avalanchego"
		}
		testFlags := utils.TestFlags{
			"avalanchego-path": avalanchegoPath,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("AvalancheGo path: %s", avalanchegoPath)))
		gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("HAPPY PATH: local deploy convert only", func() {
		testFlags := utils.TestFlags{
			"convert-only": true,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		checkConvertOnlyOutput(output, false, blockchainCmdArgs[0])
		gomega.Expect(err).Should(gomega.BeNil())
		// check that validator manager type is undefined
		sc, err := utils.GetSideCar(blockchainCmdArgs[0])
		gomega.Expect(err).Should(gomega.BeNil())
		subnetInfo, _ := blockchain.GetSubnet(sc.Networks["Local Network"].SubnetID, models.NewLocalNetwork())
		localClusterUris, err := utils.GetLocalClusterUris()
		gomega.Expect(err).Should(gomega.BeNil())
		validatorManagerAddress := "0x" + hex.EncodeToString(subnetInfo.ManagerAddress)
		uri := fmt.Sprintf("%s/ext/bc/%s/rpc", localClusterUris[0], sc.Networks["Local Network"].BlockchainID)
		valType := validatorManagerSDK.GetValidatorManagerType(uri, common.HexToAddress(validatorManagerAddress))
		expectedValType := validatormanagertypes.ValidatorManagementTypeFromString(validatormanagertypes.UndefinedValidatorManagement)
		gomega.Expect(valType).Should(gomega.Equal(expectedValType))
	})

	ginkgo.It("HAPPY PATH: generate node id ends in convert only", func() {
		testFlags := utils.TestFlags{
			"generate-node-id":         true,
			"num-bootstrap-validators": 1,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		checkConvertOnlyOutput(output, true, blockchainCmdArgs[0])
		gomega.Expect(err).Should(gomega.BeNil())
		sc, err := utils.GetSideCar(blockchainCmdArgs[0])
		gomega.Expect(err).Should(gomega.BeNil())
		numValidators := len(sc.Networks["Local Network"].BootstrapValidators)
		gomega.Expect(numValidators).Should(gomega.BeEquivalentTo(1))
		gomega.Expect(sc.Networks["Local Network"].BootstrapValidators[0].NodeID).ShouldNot(gomega.BeNil())
		gomega.Expect(sc.Networks["Local Network"].BootstrapValidators[0].BLSProofOfPossession).ShouldNot(gomega.BeNil())
		gomega.Expect(sc.Networks["Local Network"].BootstrapValidators[0].BLSPublicKey).ShouldNot(gomega.BeNil())
	})

	ginkgo.It("HAPPY PATH: local deploy with bootstrap validator balance", func() {
		testFlags := utils.TestFlags{
			"balance": 0.2,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())

		sc, err := utils.GetSideCar(blockchainCmdArgs[0])
		gomega.Expect(err).Should(gomega.BeNil())

		testFlags = utils.TestFlags{
			"local":         true,
			"validation-id": sc.Networks["Local Network"].BootstrapValidators[0].ValidationID,
		}
		output, err = utils.TestCommand(cmd.ValidatorCmd, "getBalance", nil, nil, testFlags)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(output).To(gomega.ContainSubstring("Validator Balance: 0.20000 AVAX"))
	})

	ginkgo.It("HAPPY PATH: local deploy with bootstrap filepath", func() {
		testFlags := utils.TestFlags{
			"bootstrap-filepath": utils.BootstrapValidatorPath2,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		checkConvertOnlyOutput(output, false, blockchainCmdArgs[0])
		gomega.Expect(err).Should(gomega.BeNil())

		sc, err := utils.GetSideCar(blockchainCmdArgs[0])
		gomega.Expect(err).Should(gomega.BeNil())

		for i := 0; i < 2; i++ {
			testFlags := utils.TestFlags{
				"local":         true,
				"validation-id": sc.Networks["Local Network"].BootstrapValidators[i].ValidationID,
			}
			output, err = utils.TestCommand(cmd.ValidatorCmd, "getBalance", nil, nil, testFlags)
			gomega.Expect(err).Should(gomega.BeNil())
			if i == 0 {
				sc.Networks["Local Network"].BootstrapValidators[i].NodeID = "NodeID-144PM69m93kSFyfTHMwULTmoGZSWzQ4C1"
				sc.Networks["Local Network"].BootstrapValidators[i].Weight = 20
				sc.Networks["Local Network"].BootstrapValidators[i].BLSPublicKey = "0x80b7851ce335cee149b7cfffbf6cf0bbca3c9b25026a24056e610976d095906e833a66d5ca5c56c23a3fe50e8785a81f"
				sc.Networks["Local Network"].BootstrapValidators[i].BLSProofOfPossession = "0x89e1d6d47ff04ec0c78501a029865140e9ec12baba75a95bfc5710b3fecb8db4b6cecb5ccb1136e19f88db0539deb4420306dd60145024197b41cf89179790f20146fba398bc4d13e08540ea812207f736ca007275e4ebdb840065fdb38573de"
				sc.Networks["Local Network"].BootstrapValidators[i].ChangeOwnerAddr = "P-custom1y5ku603lh583xs9v50p8kk0awcqzgeq0mezkqr"
				// we set first validator to have 0.2 AVAX balance in test_bootstrap_validator2.json
				gomega.Expect(output).To(gomega.ContainSubstring("Validator Balance: 0.20000 AVAX"))
			} else {
				sc.Networks["Local Network"].BootstrapValidators[i].NodeID = "NodeID-FtB74cdqNRrrsEpcyMHMvdpsRVodBupi3"
				sc.Networks["Local Network"].BootstrapValidators[i].Weight = 30
				sc.Networks["Local Network"].BootstrapValidators[i].BLSPublicKey = "0x8061a9d92920bff462c21318e77597ce322169eac4dce20aa842740b684d80a071be78dc56f789d3ef11f19314d871bd"
				sc.Networks["Local Network"].BootstrapValidators[i].BLSProofOfPossession = "0x83da8a3f0324ee3f23bd09adcb7d3fcd1023246ca2ead75e9d55ff1397bb1063ebd9a3c67b4042f698ac445486d0102009a206163cb80c3c92a8029c0ce2bc95d8bb6cf4af8ff5882935ae92926ca0b856fe60c62f849ee463c079aa187240ec"
				sc.Networks["Local Network"].BootstrapValidators[i].ChangeOwnerAddr = "P-custom1y5ku603lh583xs9v50p8kk0awcqzgeq0mezkqr"
				// we set second validator to have 0.3 AVAX balance in test_bootstrap_validator2.json
				gomega.Expect(output).To(gomega.ContainSubstring("Validator Balance: 0.30000 AVAX"))
			}
		}
	})

	ginkgo.It("HAPPY PATH: local deploy with change owner address", func() {
		testFlags := utils.TestFlags{
			"change-owner-address": "P-custom1y5ku603lh583xs9v50p8kk0awcqzgeq0mezkqr",
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())

		sc, err := utils.GetSideCar(blockchainCmdArgs[0])
		gomega.Expect(err).Should(gomega.BeNil())

		// get validation ID of the validator
		validators, err := utils.GetCurrentValidatorsLocalAPI(sc.Networks["Local Network"].SubnetID)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(len(validators)).Should(gomega.Equal(1))

		// verify change reward owner of the validator
		addr, _ := utils.GetL1ValidatorInfo(validators[0].ValidationID)
		gomega.Expect(addr.RemainingBalanceOwner.Addresses[0]).Should(gomega.Equal("P-custom1y5ku603lh583xs9v50p8kk0awcqzgeq0mezkqr"))
	})

	ginkgo.It("HAPPY PATH: local deploy subnet-only subnet-id flags", func() {
		testFlags := utils.TestFlags{
			"subnet-only": true,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(output).ShouldNot(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(output).ShouldNot(gomega.ContainSubstring("CreateChainTx fee"))
		gomega.Expect(output).Should(gomega.ContainSubstring("CreateSubnetTx fee"))
		gomega.Expect(err).Should(gomega.BeNil())

		// get the subnet id through reg-ex
		re := regexp.MustCompile(`Blockchain has been created with ID: (\S+)`)
		matches := re.FindStringSubmatch(output)
		gomega.Expect(len(matches)).Should(gomega.BeEquivalentTo(2))

		// no local machine validators should have been created
		_, err = utils.GetLocalClusterUris()
		gomega.Expect(err).Should(gomega.MatchError("expected 1 local network cluster running, found 0"))

		subnetID := matches[1]
		testFlags = utils.TestFlags{
			"subnet-id": subnetID,
		}

		output, err = utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(output).Should(gomega.ContainSubstring("CreateChainTx fee"))
		gomega.Expect(output).ShouldNot(gomega.ContainSubstring("CreateSubnetTx fee"))
		gomega.Expect(err).Should(gomega.BeNil())

		sc, err := utils.GetSideCar(blockchainCmdArgs[0])
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(sc.Networks["Local Network"].SubnetID.String()).Should(gomega.BeEquivalentTo(subnetID))

		// no local machine validators should have been created
		localClusterUris, err := utils.GetLocalClusterUris()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(len(localClusterUris)).Should(gomega.Equal(1))
	})

	ginkgo.It("HAPPY PATH: local deploy with signature aggregator endpoint set", func() {
		_, err := utils.TestCommand(cmd.NetworkCmd, "start", nil, nil, nil)
		gomega.Expect(err).Should(gomega.BeNil())
		listSigAggCmd := exec.Command("./bin/avalanche", "interchain", "signatureAggregator", "start", "--local")
		_, err = listSigAggCmd.CombinedOutput()
		gomega.Expect(err).Should(gomega.BeNil())

		app := utils.GetApp()
		runFilePath := app.GetLocalSignatureAggregatorRunPath(models.Local)
		signatureAggregatorEndpoint := ""
		// Check if run file exists and read ports from it
		if _, err := os.Stat(runFilePath); err == nil {
			// File exists, get process details
			runFile, err := signatureaggregator.GetCurrentSignatureAggregatorProcessDetails(app, models.NewLocalNetwork())
			gomega.Expect(err).Should(gomega.BeNil())
			signatureAggregatorEndpoint = fmt.Sprintf("http://localhost:%d/aggregate-signatures", runFile.APIPort)
		}
		testFlags := utils.TestFlags{
			"signature-aggregator-endpoint": signatureAggregatorEndpoint,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("HAPPY PATH: local deploy with signature aggregator previously started", func() {
		_, err := utils.TestCommand(cmd.NetworkCmd, "start", nil, nil, nil)
		gomega.Expect(err).Should(gomega.BeNil())
		listSigAggCmd := exec.Command("./bin/avalanche", "interchain", "signatureAggregator", "start", "--local")
		_, err = listSigAggCmd.CombinedOutput()
		gomega.Expect(err).Should(gomega.BeNil())

		testFlags := utils.TestFlags{}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(output).Should(gomega.ContainSubstring("L1 is successfully deployed on Local Network"))
		gomega.Expect(err).Should(gomega.BeNil())
	})

	ginkgo.It("HAPPY PATH: local deploy set num bootstrap validators", func() {
		testFlags := utils.TestFlags{
			"num-bootstrap-validators": 2,
		}
		_, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.BeNil())

		sc, err := utils.GetSideCar(blockchainCmdArgs[0])
		gomega.Expect(err).Should(gomega.BeNil())
		numValidators := len(sc.Networks["Local Network"].BootstrapValidators)
		gomega.Expect(numValidators).Should(gomega.BeEquivalentTo(2))

		localClusterUris, err := utils.GetLocalClusterUris()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(len(localClusterUris)).Should(gomega.Equal(2))
	})

	ginkgo.It("ERROR PATH: invalid_version", func() {
		testFlags := utils.TestFlags{
			"avalanchego-version": "invalid_version",
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("invalid version string"))
	})

	ginkgo.It("ERROR PATH: invalid_avalanchego_path", func() {
		avalancheGoPath := "invalid_avalanchego_path"
		testFlags := utils.TestFlags{
			"avalanchego-path": avalancheGoPath,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring(fmt.Sprintf("avalancheGo binary %s does not exist", avalancheGoPath)))
	})

	ginkgo.It("ERROR PATH: zero balance value", func() {
		testFlags := utils.TestFlags{
			"balance": 0,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("bootstrap validator balance must be greater than 0 AVAX"))
	})

	ginkgo.It("ERROR PATH: negative balance value", func() {
		testFlags := utils.TestFlags{
			"balance": -1.0,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("bootstrap validator balance must be greater than 0 AVAX"))
	})

	ginkgo.It("ERROR PATH: invalid bootstrap filepath", func() {
		fileName := "nonexistent.json"
		testFlags := utils.TestFlags{
			"bootstrap-filepath": fileName,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("file path \"%s\" doesn't exist", fileName))
	})

	ginkgo.It("ERROR PATH: invalid change owner address format", func() {
		testFlags := utils.TestFlags{
			"change-owner-address": ewoqEVMAddress,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("failure parsing change owner address: no separator found in address"))
	})

	ginkgo.It("ERROR PATH: generate node id is not applicable if convert only is false", func() {
		testFlags := utils.TestFlags{
			"generate-node-id": true,
			"convert-only":     false,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("cannot set --convert-only=false if --generate-node-id=true"))
	})
	ginkgo.It("ERROR PATH: generate node id is not applicable if use local machine is true", func() {
		testFlags := utils.TestFlags{
			"generate-node-id":  true,
			"use-local-machine": true,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("cannot use local machine as bootstrap validator if --generate-node-id=true"))
	})

	ginkgo.It("ERROR PATH: bootstrap filepath is not applicable if convert only is false", func() {
		testFlags := utils.TestFlags{
			"bootstrap-filepath": utils.BootstrapValidatorPath,
			"convert-only":       false,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("cannot set --convert-only=false if --bootstrap-filepath is not empty"))
	})
	ginkgo.It("ERROR PATH: bootstrap filepath is not applicable if use local machine is true", func() {
		testFlags := utils.TestFlags{
			"bootstrap-filepath": utils.BootstrapValidatorPath,
			"use-local-machine":  true,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("cannot use local machine as bootstrap validator if --bootstrap-filepath is not empty"))
	})
	ginkgo.It("ERROR PATH: bootstrap endpoints is not applicable if convert only is false", func() {
		testFlags := utils.TestFlags{
			"bootstrap-endpoints": "127.0.0.1:9650",
			"convert-only":        false,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("cannot set --convert-only=false if --bootstrap-endpoints is not empty"))
	})
	ginkgo.It("ERROR PATH: bootstrap endpoints is not applicable if use local machine is true", func() {
		testFlags := utils.TestFlags{
			"bootstrap-endpoints": "127.0.0.1:9650",
			"use-local-machine":   true,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("cannot use local machine as bootstrap validator if --bootstrap-endpoints is not empty"))
	})
	ginkgo.It("ERROR PATH: bootstrap filepath cannot be set if generate node id is true", func() {
		testFlags := utils.TestFlags{
			"bootstrap-filepath": utils.BootstrapValidatorPath2,
			"generate-node-id":   true,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("cannot use --generate-node-id=true and a non-empty --bootstrap-filepath at the same time"))
	})
	ginkgo.It("ERROR PATH: bootstrap filepath cannot be set if num bootstrap validators is set", func() {
		testFlags := utils.TestFlags{
			"bootstrap-filepath":       utils.BootstrapValidatorPath2,
			"num-bootstrap-validators": 2,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("cannot use a non-empty --num-bootstrap-validators and a non-empty --bootstrap-filepath at the same time"))
	})
	ginkgo.It("ERROR PATH: bootstrap filepath cannot be set if balance is set", func() {
		testFlags := utils.TestFlags{
			"bootstrap-filepath": utils.BootstrapValidatorPath2,
			"balance":            0.2,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("cannot use a non-empty --balance and a non-empty --bootstrap-filepath at the same time"))
	})
	ginkgo.It("ERROR PATH: bootstrap filepath cannot be set if bootstrap endpoints is set", func() {
		testFlags := utils.TestFlags{
			"bootstrap-filepath":  utils.BootstrapValidatorPath2,
			"bootstrap-endpoints": "127.0.0.1:9650",
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("cannot use a non-empty --bootstrap-endpoints and a non-empty --bootstrap-filepath at the same time"))
	})
	ginkgo.It("ERROR PATH: bootstrap endpoints is not applicable if generate node id is true", func() {
		testFlags := utils.TestFlags{
			"bootstrap-endpoints": utils.BootstrapValidatorPath2,
			"generate-node-id":    true,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("cannot use --generate-node-id=true and a non-empty --bootstrap-endpoints at the same time"))
	})
	ginkgo.It("ERROR PATH: bootstrap endpoints is not applicable if num bootstrap validators is set", func() {
		testFlags := utils.TestFlags{
			"bootstrap-endpoints":      utils.BootstrapValidatorPath2,
			"num-bootstrap-validators": 2,
		}
		output, err := utils.TestCommand(cmd.BlockchainCmd, "deploy", blockchainCmdArgs, globalFlags, testFlags)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		gomega.Expect(output).Should(gomega.ContainSubstring("cannot use a non-empty --num-bootstrap-validators and a non-empty --bootstrap-endpoints at the same time"))
	})
})
