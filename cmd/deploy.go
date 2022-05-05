/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/cmd/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/docker/docker/pkg/reexec"
	"github.com/spf13/cobra"
	// "github.com/ava-labs/avalanche-network-runner/cmd/avalanche-network-runner/server"
)

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy your subnet to a network",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: deploySubnet,
	Args: cobra.ExactArgs(1),
}

var (
	deployLocal *bool
	force       *bool
)

func init() {
	subnetCmd.AddCommand(deployCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// deployCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	deployLocal = deployCmd.Flags().BoolP("local", "l", false, "Deploy subnet locally")
	force = deployCmd.Flags().BoolP("force", "f", false, "Deploy without asking for confirmation")
}

func getChainsInSubnet(subnetName string) ([]string, error) {
	usr, _ := user.Current()
	mainDir := filepath.Join(usr.HomeDir, BaseDir)
	files, err := ioutil.ReadDir(mainDir)
	if err != nil {
		return []string{}, err
	}

	chains := []string{}

	for _, f := range files {
		if strings.Contains(f.Name(), sidecar_suffix) {
			// read in sidecar file
			path := filepath.Join(mainDir, f.Name())
			jsonBytes, err := os.ReadFile(path)
			if err != nil {
				return []string{}, err
			}

			var sc models.Sidecar
			err = json.Unmarshal(jsonBytes, &sc)
			if err != nil {
				return []string{}, err
			}
			if sc.Subnet == subnetName {
				chains = append(chains, sc.Name)
			}
		}
	}
	return chains, nil
}

func deploySubnet(cmd *cobra.Command, args []string) error {
	// Check subnet exists
	// TODO create a file that lists chains by subnet for fast querying
	chains, err := getChainsInSubnet(args[0])
	if err != nil {
		return err
	}

	if len(chains) == 0 {
		return fmt.Errorf("Invalid subnet: %s", args[0])
	}

	var network models.Network
	if *deployLocal {
		network = models.Local
	} else {
		networkStr, err := prompts.CaptureList(
			"Choose a network to deploy on",
			[]string{models.Local.String(), models.Fuji.String(), models.Mainnet.String()},
		)
		if err != nil {
			return err
		}
		network = models.NetworkFromString(networkStr)
	}

	fmt.Println("Deploying", chains, "to", network.String())
	// TODO
	switch network {
	case models.Local:
		// WRITE CODE HERE
		fmt.Println("Deploy local")
		return deployToLocalNetwork(chains[0])
	default:
		return errors.New("Not implemented")
	}
}

func deployToLocalNetwork(chain string) error {
	isRunning, err := IsServerProcessRunning()
	if err != nil {
		return err
	}
	if !isRunning {
		fmt.Println("gRPC server is not running")
		if err := startServerProcess(); err != nil {
			return err
		}
	}
	return doDeploy(chain)
}

func doDeploy(chain string) error {
	//	curl -X POST -k http://localhost:8081/v1/control/start -d '{"execPath":"'${AVALANCHEGO_EXEC_PATH}'","numNodes":5,"logLevel":"INFO","pluginDir":"'${AVALANCHEGO_PLUGIN_PATH}'","customVms":{"subnetevm":"/tmp/subnet-evm.genesis.json"}}'
	requestTimeout := 3 * time.Minute

	cli, err := client.New(client.Config{
		LogLevel:    "info",
		Endpoint:    "0.0.0.0:8097",
		DialTimeout: 10 * time.Second,
	})
	if err != nil {
		return err
	}
	defer cli.Close()

	pluginDir := "/home/fabio/go/src/github.com/ava-labs/avalanchego/build/plugins"
	avalancheGoBinPath := "/home/fabio/go/src/github.com/ava-labs/avalanchego/build/avalanchego"

	chain_genesis := fmt.Sprintf("/home/fabio/.avalanche-cli/%s_genesis.json", chain)

	customVMs := map[string]string{
		chain: chain_genesis,
	}

	opts := []client.OpOption{
		// client.WithNumNodes(numNodes),
		client.WithPluginDir(pluginDir),
		client.WithCustomVMs(customVMs),
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	// don't call since "start" is async
	// and the top-level context here "ctx" is passed
	// to all underlying function calls
	// just set the timeout to halt "Start" async ops
	// when the deadline is reached
	_ = cancel

	vmID, err := utils.VMID("subnet-evm")
	if err != nil {
		return err
	}
	fmt.Printf("this VM will get ID: %s\n", vmID.String())
	if err := buildVM(chain, vmID, pluginDir); err != nil {
		return err
	}

	fmt.Println("VM ready. Trying to boot network...")
	info, err := cli.Start(
		ctx,
		avalancheGoBinPath,
		opts...,
	)
	if err != nil {
		return err
	}
	fmt.Println("network is up and running")
	fmt.Println(info)
	return nil
}

func getVMBinary(id ids.ID) error {
	return nil
}

// this is NOT viable. Too many things can go wrong.
// i.e. cloning with git might require the ssh password...
func buildVM(chain string, id ids.ID, pluginDir string) error {
	fmt.Println("cloning subnet-evm...")
	subnetEVMRepo := "https://github.com/ava-labs/subnet-evm"
	// fatal: destination path '/home/fabio/go/src/github.com/ava-labs' already exists and is not an empty directory.
	// dest := "$HOME/go/src/github.com/ava-labs"
	dest := "/tmp/subnet-evm"
	args := []string{"clone", subnetEVMRepo, dest}
	// TODO git could not be installed...we need binaries
	clone := exec.Command("git", args...)
	clone.Stdout = os.Stdout
	clone.Stderr = os.Stderr
	if err := clone.Run(); err != nil {
		return err
	}

	fmt.Println("done. building...")
	// subnetEVMPath := "$HOME/go/src/github.com/ava-labs/subnet-evm/"
	buildPath := filepath.Join(dest, "scripts/build.sh")
	buildDest := filepath.Join("build", id.String())
	build := exec.Command(buildPath, buildDest)
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return err
	}
	fmt.Println("done. copying to avalanchego plugin path...")
	binPath := filepath.Join(dest, buildDest)
	cpArgs := []string{binPath, pluginDir}
	cp := exec.Command("cp", cpArgs...)
	cp.Stdout = os.Stdout
	cp.Stderr = os.Stderr
	if err := cp.Run(); err != nil {
		return err
	}
	fmt.Println("all good")
	return nil
}

func startServerProcess() error {
	thisBin := reexec.Self()

	args := []string{"backend", "start"}
	cmd := exec.Command(thisBin, args...)
	outputFile, err := os.CreateTemp("", "avalanche-cli-backend*")
	if err != nil {
		return err
	}
	cmd.Stdout = outputFile
	cmd.Stderr = outputFile

	if err := cmd.Start(); err != nil {
		return err
	}
	fmt.Printf("Backend controller started, pid: %d, output at: %s\n", cmd.Process.Pid, outputFile.Name())
	return nil
}
