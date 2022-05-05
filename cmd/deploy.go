/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
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
	"github.com/ava-labs/avalanchego/utils/perms"
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

const (
	binaryServerURL = "http://3.84.91.164:8998"
	serverRun       = "/tmp/gRPCserver.run"
)

var (
	deployLocal         *bool
	force               *bool
	localgRPCOutputFile string
	localgRPCPid        int
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
	// if err := buildVM(chain, vmID, pluginDir); err != nil {
	if err := getVMBinary(vmID, pluginDir); err != nil {
		return err
	}

	fmt.Println("VM ready. Trying to boot network...")
	info, err := cli.Start(
		ctx,
		avalancheGoBinPath,
		opts...,
	)
	if err != nil {
		fmt.Printf("failed to start network :%s\n", err)
		return err
	}

	fmt.Println(info)
	fmt.Println("network has been booted. wait until healthy...")

	if err := waitForHealthy(cli, ctx); err != nil {
		fmt.Printf("failed to query network health: %s\n", err)
		return err
	}

	uris, err := cli.URIs(ctx)
	if err != nil {
		fmt.Printf("failed to query uri endpoints: %s\n", err)
		return err
	}

	fmt.Println("network ready to use. local network node endpoints:")
	for _, u := range uris {
		fmt.Println(u)
	}
	return nil
}

func waitForHealthy(cli client.Client, ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
			fmt.Println("polling for health...")
			_, err := cli.Health(ctx)
			if err != nil {
				fmt.Println("not yet")
				continue
			}
			fmt.Println("all good!")
			return nil
		}
	}
}

func getVMBinary(id ids.ID, pluginDir string) error {
	vmID := id.String()
	binaryPath := filepath.Join(pluginDir, vmID)
	info, err := os.Stat(binaryPath)
	if err == nil {
		if !info.IsDir() {
			fmt.Println("binary already exists, skipping download")
		}
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	fmt.Println("binary does not exist locally, starting download...")

	base, err := url.Parse(binaryServerURL)
	if err != nil {
		return err
	}

	// Path params
	// base.Path += "this will get automatically encoded"

	// Query params
	params := url.Values{}
	params.Add("vmid", vmID)
	base.RawQuery = params.Encode()

	fmt.Printf("starting download from %s...\n\n", base.String())

	resp, err := http.Get(base.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		fmt.Println("download successful. installing binary...")
		return installBinary(bodyBytes, binaryPath)

	} else {
		return fmt.Errorf("downloading binary failed, status code: %d", resp.StatusCode)
	}
}

func installBinary(binary []byte, binaryPath string) error {
	if err := os.WriteFile(binaryPath, binary, perms.ReadWriteExecute); err != nil {
		return err
	}
	fmt.Println("binary installed. ready to go.")
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
	content := fmt.Sprintf("gRPC server output file: %s\ngRPC server PID: %d\n", outputFile.Name(), cmd.Process.Pid)
	err = os.WriteFile(serverRun, []byte(content), perms.ReadWrite)
	if err != nil {
		fmt.Printf("WARN: Could not write gRPC process info to file: %s\n", err)
	}
	return nil
}
