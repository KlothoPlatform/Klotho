package main

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	pb "github.com/klothoplatform/klotho/pkg/k2/language_host/go"
	"github.com/klothoplatform/klotho/pkg/logging"
	"go.uber.org/zap"
)

func healthCheck(client pb.KlothoServiceClient) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	return err == nil
}

func waitForServer(client pb.KlothoServiceClient, retries int, delay time.Duration) error {
	for i := 0; i < retries; i++ {
		if healthCheck(client) {
			return nil
		}
		time.Sleep(delay)
	}
	return fmt.Errorf("server not available after %d retries", retries)
}

func startPythonClient() *exec.Cmd {
	cmd := logging.Command(
		context.TODO(),
		logging.CommandLogger{RootLogger: zap.L().Named("python")},
		"pipenv", "run", "python", "python_language_host.py",
	)
	cmd.Dir = "pkg/k2/language_host/python"
	// spawn the python process as a subprocess of the CLI so it is guaranteed to be killed when the CLI exits
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	zap.S().Debugf("Executing: %s for %v", cmd.Path, cmd.Args)
	if err := cmd.Start(); err != nil {
		zap.S().Fatalf("failed to start Python client: %v", err)
	}
	zap.L().Info("Python client started")

	go func() {
		err := cmd.Wait()
		if err != nil {
			zap.S().Errorf("Python process exited with error: %v", err)
		} else {
			zap.L().Debug("Python process exited successfully")
		}
	}()
	return cmd
}
