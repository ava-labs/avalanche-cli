// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package subnet

import (
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/tests/e2e/commands"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	CLIBinary         = "./bin/avalanche"
	keyName           = "ewoq"
	ewoqEVMAddress    = "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
	ewoqPChainAddress = "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p"
)

var (
	blockchainID                       string
	localClusterUris                   []string
	avagoVersion                       string
	rpcEndpoint                        string
	validatorManagerAddress            string
	specializedValidatorManagerAddress string
)

var _ = ginkgo.Describe("[AddRemove Validator SOV L1 Manager PoS Native]", func() {
	ginkgo.It("Create Etna Subnet Config", func() {
		_, avagoVersion = commands.CreateEtnaSubnetEvmConfig(
			utils.BlockchainName,
			ewoqEVMAddress,
			commands.PoSNative,
		)
	})

	ginkgo.It("Can create an Etna Local Network", func() {
		output := commands.StartNetworkWithVersion(avagoVersion)
		fmt.Println(output)
	})

	ginkgo.It("Can create a local node connected to Etna Local Network", func() {
		output, err := commands.CreateLocalEtnaNode(
			avagoVersion,
			utils.TestLocalNodeName,
			7,
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
		localClusterUris, err = utils.GetLocalClusterUris()
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(len(localClusterUris)).Should(gomega.Equal(7))
	})

	ginkgo.It("Deploy Etna Subnet", func() {
		output, err := commands.DeployEtnaBlockchain(
			utils.BlockchainName,
			utils.TestLocalNodeName,
			[]string{
				localClusterUris[0],
				localClusterUris[1],
				localClusterUris[2],
				localClusterUris[3],
				localClusterUris[4],
			},
			ewoqPChainAddress,
			true,  // convertOnly
			false, // externalManager
			"",    // erc20TokenAddress
			0,     // rewardBasisPoints (L1 Manager: reward calculator deployed during initValidatorManager)
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can make cluster track a subnet", func() {
		output, err := commands.TrackLocalEtnaSubnet(utils.TestLocalNodeName, utils.BlockchainName)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
		// parse blockchainID from output
		re := regexp.MustCompile(`Waiting for blockchain ([A-Za-z0-9]+) to be bootstrapped`)
		// Find the first match
		match := re.FindStringSubmatch(output)
		gomega.Expect(match).ToNot(gomega.BeEmpty())
		if len(match) > 1 {
			// The first submatch will contain the chain ID
			blockchainID = match[1]
		}
		gomega.Expect(blockchainID).Should(gomega.Not(gomega.BeEmpty()))
		ginkgo.GinkgoWriter.Printf("Blockchain ID: %s\n", blockchainID)
	})

	ginkgo.It("Can initialize a PoS Manager contract", func() {
		output, err := commands.InitValidatorManager(
			utils.BlockchainName,
			utils.TestLocalNodeName,
			"",
			blockchainID,
			500000000, // 5,000,000% APR for testing
			"",        // erc20TokenAddress (not used for PoS Native)
			"",        // rewardCalculatorAddress (deployed by init for L1 Manager)
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)

		// Get RPC endpoint and validator manager address from sidecar
		sidecar, err := utils.GetSideCar(utils.BlockchainName)
		gomega.Expect(err).Should(gomega.BeNil())
		networkData := sidecar.Networks[models.NewLocalNetwork().Name()]
		rpcEndpoint = networkData.ValidatorManagerRPCEndpoint
		validatorManagerAddress = networkData.ValidatorManagerAddress
		specializedValidatorManagerAddress = networkData.SpecializedValidatorManagerAddress
		gomega.Expect(rpcEndpoint).Should(gomega.Not(gomega.BeEmpty()))
		gomega.Expect(validatorManagerAddress).Should(gomega.Not(gomega.BeEmpty()))
		gomega.Expect(specializedValidatorManagerAddress).Should(gomega.Not(gomega.BeEmpty()))
		fmt.Printf("RPC Endpoint: %s\n", rpcEndpoint)
		fmt.Printf("Validator Manager Address: %s\n", validatorManagerAddress)
		fmt.Printf("Specialized Validator Manager Address: %s\n", specializedValidatorManagerAddress)
	})

	ginkgo.It("Can't add validator with too much of a weight", func() {
		output, err := commands.AddEtnaSubnetValidatorToCluster(
			utils.TestLocalNodeName,
			utils.BlockchainName,
			localClusterUris[5],
			ewoqPChainAddress,
			1,
			false, // use existing
			200,
		)
		gomega.Expect(err).Should(gomega.HaveOccurred())
		fmt.Println(output)
	})

	ginkgo.It("Can add validator", func() {
		stakerBalanceBefore, err := utils.GetNativeBalance(rpcEndpoint, ewoqEVMAddress)
		gomega.Expect(err).Should(gomega.BeNil())
		specializedVMBalanceBefore, err := utils.GetNativeBalance(rpcEndpoint, specializedValidatorManagerAddress)
		gomega.Expect(err).Should(gomega.BeNil())

		output, err := commands.AddEtnaSubnetValidatorToCluster(
			utils.TestLocalNodeName,
			utils.BlockchainName,
			localClusterUris[5],
			ewoqPChainAddress,
			1,
			false, // use existing
			20,
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)

		// Check balances after adding validator
		stakerBalanceAfter, err := utils.GetNativeBalance(rpcEndpoint, ewoqEVMAddress)
		gomega.Expect(err).Should(gomega.BeNil())
		specializedVMBalanceAfter, err := utils.GetNativeBalance(rpcEndpoint, specializedValidatorManagerAddress)
		gomega.Expect(err).Should(gomega.BeNil())

		// Verify balance changes (stake is 20 tokens with 18 decimals = 20 * 10^18)
		expectedStake := new(big.Int).Mul(big.NewInt(20), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
		expectedStakerBalance := new(big.Int).Sub(stakerBalanceBefore, expectedStake)
		expectedSpecializedVMBalance := new(big.Int).Add(specializedVMBalanceBefore, expectedStake)

		// Allow for gas costs (max ~0.1 tokens = 0.1 * 10^18)
		maxGasCost := new(big.Int).Mul(big.NewInt(1), new(big.Int).Exp(big.NewInt(10), big.NewInt(17), nil))

		// Staker balance should be close to expected (within gas cost range)
		// Check: 0 <= stakerDiff <= maxGasCost
		stakerDiff := new(big.Int).Sub(expectedStakerBalance, stakerBalanceAfter)
		gomega.Expect(stakerDiff.Cmp(big.NewInt(0))).Should(gomega.BeNumerically(">=", 0))
		gomega.Expect(stakerDiff.Cmp(maxGasCost)).Should(gomega.BeNumerically("<=", 0))

		// Specialized VM balance should be exactly as expected (no gas costs for receiving)
		gomega.Expect(specializedVMBalanceAfter).Should(gomega.Equal(expectedSpecializedVMBalance))
	})

	ginkgo.It("Can add second validator", func() {
		stakerBalanceBefore, err := utils.GetNativeBalance(rpcEndpoint, ewoqEVMAddress)
		gomega.Expect(err).Should(gomega.BeNil())
		specializedVMBalanceBefore, err := utils.GetNativeBalance(rpcEndpoint, specializedValidatorManagerAddress)
		gomega.Expect(err).Should(gomega.BeNil())

		output, err := commands.AddEtnaSubnetValidatorToCluster(
			utils.TestLocalNodeName,
			utils.BlockchainName,
			localClusterUris[6],
			ewoqPChainAddress,
			1,
			false, // use existing
			20,
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)

		// Check balances after adding validator
		stakerBalanceAfter, err := utils.GetNativeBalance(rpcEndpoint, ewoqEVMAddress)
		gomega.Expect(err).Should(gomega.BeNil())
		specializedVMBalanceAfter, err := utils.GetNativeBalance(rpcEndpoint, specializedValidatorManagerAddress)
		gomega.Expect(err).Should(gomega.BeNil())

		// Verify balance changes (stake is 20 tokens with 18 decimals = 20 * 10^18)
		expectedStake := new(big.Int).Mul(big.NewInt(20), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
		expectedStakerBalance := new(big.Int).Sub(stakerBalanceBefore, expectedStake)
		expectedSpecializedVMBalance := new(big.Int).Add(specializedVMBalanceBefore, expectedStake)

		// Allow for gas costs (max ~0.1 tokens = 0.1 * 10^18)
		maxGasCost := new(big.Int).Mul(big.NewInt(1), new(big.Int).Exp(big.NewInt(10), big.NewInt(17), nil))

		// Staker balance should be close to expected (within gas cost range)
		// Check: 0 <= stakerDiff <= maxGasCost
		stakerDiff := new(big.Int).Sub(expectedStakerBalance, stakerBalanceAfter)
		gomega.Expect(stakerDiff.Cmp(big.NewInt(0))).Should(gomega.BeNumerically(">=", 0))
		gomega.Expect(stakerDiff.Cmp(maxGasCost)).Should(gomega.BeNumerically("<=", 0))

		// Specialized VM balance should be exactly as expected (no gas costs for receiving)
		gomega.Expect(specializedVMBalanceAfter).Should(gomega.Equal(expectedSpecializedVMBalance))
	})

	ginkgo.It("Can get status of thecluster", func() {
		output, err := commands.GetLocalClusterStatus(utils.TestLocalNodeName, utils.BlockchainName)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
		// make sure we can find string with "http://127.0.0.1:port" and "L1:Validating" string in the output
		parsedURL, err := url.Parse(localClusterUris[1])
		gomega.Expect(err).Should(gomega.BeNil())
		port := parsedURL.Port()
		gomega.Expect(port).Should(gomega.Not(gomega.BeEmpty()))
		regexp := fmt.Sprintf(`http://127\.0\.0\.1:%s.*Validating`, port)
		gomega.Expect(output).To(gomega.MatchRegexp(regexp), fmt.Sprintf("expect to have L1 validated by port %s", port))
		parsedURL, err = url.Parse(localClusterUris[2])
		gomega.Expect(err).Should(gomega.BeNil())
		port = parsedURL.Port()
		gomega.Expect(port).Should(gomega.Not(gomega.BeEmpty()))
		regexp = fmt.Sprintf(`http://127\.0\.0\.1:%s.*Validating`, port)
		gomega.Expect(output).To(gomega.MatchRegexp(regexp), fmt.Sprintf("expect to have L1 validated by port %s", port))
	})

	ginkgo.It("Can sleep for min stake duration", func() {
		time.Sleep(2 * time.Minute)
	})

	ginkgo.It("Can remove non-bootstrap validator", func() {
		stakerBalanceBefore, err := utils.GetNativeBalance(rpcEndpoint, ewoqEVMAddress)
		gomega.Expect(err).Should(gomega.BeNil())
		specializedVMBalanceBefore, err := utils.GetNativeBalance(rpcEndpoint, specializedValidatorManagerAddress)
		gomega.Expect(err).Should(gomega.BeNil())

		output, err := commands.RemoveEtnaSubnetValidatorFromCluster(
			utils.TestLocalNodeName,
			utils.BlockchainName,
			localClusterUris[5],
			keyName,
			0,
			false, // non-bootstrap validator, no force needed
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)

		// Check balances after removing validator
		stakerBalanceAfter, err := utils.GetNativeBalance(rpcEndpoint, ewoqEVMAddress)
		gomega.Expect(err).Should(gomega.BeNil())
		specializedVMBalanceAfter, err := utils.GetNativeBalance(rpcEndpoint, specializedValidatorManagerAddress)
		gomega.Expect(err).Should(gomega.BeNil())

		// Constants for balance verification
		stakeAmount := new(big.Int).Mul(big.NewInt(20), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)) // 20 tokens
		maxGasCost := new(big.Int).Mul(big.NewInt(1), new(big.Int).Exp(big.NewInt(10), big.NewInt(17), nil))   // 0.1 tokens
		// minExpectedReward is 3.5 tokens (conservative estimate)
		// Based on: 20 tokens stake, 500000000 basis points (5,000,000% APR), 120 seconds duration
		// Formula: (20 * 500000000 * 120) / 31536000 / 10000 ≈ 3.805 tokens
		minExpectedReward := new(big.Int).Mul(big.NewInt(35), new(big.Int).Exp(big.NewInt(10), big.NewInt(17), nil)) // 3.5 tokens

		// Minimum expected staker balance: stakerBalanceBefore + stakeAmount + minExpectedReward - maxGasCost
		minExpectedStakerBalance := new(big.Int).Add(stakerBalanceBefore, stakeAmount)
		minExpectedStakerBalance.Add(minExpectedStakerBalance, minExpectedReward)
		minExpectedStakerBalance.Sub(minExpectedStakerBalance, maxGasCost)

		// Staker should have at least stake + minimum reward back (minus gas)
		gomega.Expect(stakerBalanceAfter.Cmp(minExpectedStakerBalance)).Should(gomega.BeNumerically(">=", 0))

		// Specialized VM balance should decrease by exactly stake amount
		expectedSpecializedVMBalance := new(big.Int).Sub(specializedVMBalanceBefore, stakeAmount)
		gomega.Expect(specializedVMBalanceAfter).Should(gomega.Equal(expectedSpecializedVMBalance))
	})

	ginkgo.It("Can remove bootstrap validator", func() {
		output, err := commands.RemoveEtnaSubnetValidatorFromCluster(
			utils.TestLocalNodeName,
			utils.BlockchainName,
			localClusterUris[2],
			keyName,
			0,
			true, // bootstrap validator, needs force
		)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can destroy local node", func() {
		output, err := commands.DestroyLocalNode(utils.TestLocalNodeName)
		gomega.Expect(err).Should(gomega.BeNil())
		fmt.Println(output)
	})

	ginkgo.It("Can destroy Etna Local Network", func() {
		commands.CleanNetwork()
	})

	ginkgo.It("Can remove Etna Subnet Config", func() {
		commands.DeleteSubnetConfig(utils.BlockchainName)
	})
})
