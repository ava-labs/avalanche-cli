package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cleanCmd)
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean up your deploy",
	Long:  `Cleans up your deploys including server processes`,

	Run:  clean,
	Args: cobra.ExactArgs(0),
}

func clean(cmd *cobra.Command, args []string) {
	fmt.Println("killing gRPC server process...")
	if err := killgRPCServerProcess(); err != nil {
		fmt.Printf("WARN: failed killing server process: %s\n", err)
	}
	fmt.Println("process terminated.")
}

func killgRPCServerProcess() error {
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

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	// don't call since "start" is async
	// and the top-level context here "ctx" is passed
	// to all underlying function calls
	// just set the timeout to halt "Start" async ops
	// when the deadline is reached
	_ = cancel

	_, err = cli.Stop(ctx)
	if err != nil {
		fmt.Printf("failed stopping server process: %s\n", err)
	}

	runFile, err := os.ReadFile(serverRun)
	if err != nil {
		fmt.Printf("failed reading process info file at %s: %s\n", serverRun, err)
		return err
	}
	str := string(runFile)
	pidIndex := strings.Index(str, "PID:")
	pidStart := pidIndex + len("PID: ")
	pidstr := str[pidStart:strings.LastIndex(str, "\n")]
	pid, err := strconv.Atoi(strings.TrimSpace(pidstr))
	if err != nil {
		fmt.Printf("failed reading pid from info file at %s: %s\n", serverRun, err)
		return err
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Printf("could not find process with pid %d: %s\n", pid, err)
		return err
	}
	if err := proc.Kill(); err != nil {
		fmt.Printf("failed killing process with pid %d: %s\n", pid, err)
		return err
	}

	return nil
}
