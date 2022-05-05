package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ava-labs/avalanche-network-runner/server"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// deployCmd represents the deploy command
var backendCmd = &cobra.Command{
	Use:   "backend",
	Short: "Deploy your subnet to a network",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: backendController,
	Args: cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(backendCmd)

	// Here you will define your flags and configuration settings.
}

func backendController(cmd *cobra.Command, args []string) error {
	fmt.Println("con")
	if args[0] == "start" {
		return startBackend(cmd)
	}
	return fmt.Errorf("Unsupported command")
}

func startBackend(cmd *cobra.Command) error {
	fmt.Println("start")
	s, err := server.New(server.Config{
		Port:        ":8097",
		GwPort:      ":8098",
		DialTimeout: 10 * time.Second,
	})
	if err != nil {
		return err
	}

	rootCtx, rootCancel := context.WithCancel(context.Background())
	errc := make(chan error)
	fmt.Println("starting server")
	go watchServerProcess(rootCancel, errc)
	errc <- s.Run(rootCtx)

	return nil
}

func watchServerProcess(rootCancel context.CancelFunc, errc chan error) {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-sigc:
		zap.L().Warn("signal received; closing server", zap.String("signal", sig.String()))
		rootCancel()
		zap.L().Warn("closed server", zap.Error(<-errc))
	case err := <-errc:
		zap.L().Warn("server closed", zap.Error(err))
		rootCancel()
	}
}
