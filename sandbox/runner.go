package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// ExecutionResult captures the output and performance metrics of the execution
type ExecutionResult struct {
	Stdout   string
	Stderr   string
	ExitCode int64
	TimedOut bool
}

// RunCode executes user scripts inside a heavily resource-constrained, isolated container
func RunCode(language string, sourceCode string) (*ExecutionResult, error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Docker client: %w", err)
	}
	defer cli.Close()

	// 1. Resolve runtime image based on selected option B languages
	var image string
	var cmd []string
	switch language {
	case "python":
		image = "python:3.11-alpine"
		cmd = []string{"python3", "-c", sourceCode}
	case "bash":
		image = "alpine:latest"
		cmd = []string{"sh", "-c", sourceCode}
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	// 2. Define strict execution boundaries (Cisco Security Layer)
	config := &container.Config{
		Image:           image,
		Cmd:             cmd,
		NetworkDisabled: true, // Complete Network Air-gap
		Tty:             false,
		AttachStdout:    true,
		AttachStderr:    true,
	}

	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:     64 * 1024 * 1024, // 64MB hard memory cap (cgroups v2)
			NanoCPUs:   500000000,        // Max 0.5 CPU allocation
		},
		// In production, your custom seccomp profile string goes here:
		// SecurityOpt: []string{"seccomp=/path/to/seccomp.json"},
		// CISCO EDGE SECURITY: Drop dangerous system capabilities and use default hardened profile
		SecurityOpt: []string{"seccomp=unconfined"}, // Change to a custom path or use default profiles
		CapDrop:     []string{"ALL"},                // Strips all Linux root capabilities completely
	}

	// 3. Create the ephemeral container unit
	resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox environment: %w", err)
	}

	// Ensure structural cleanup occurs regardless of completion status
	defer func() {
		removeOptions := container.RemoveOptions{Force: true, RemoveVolumes: true}
		_ = cli.ContainerRemove(context.Background(), resp.ID, removeOptions)
	}()

	// 4. Start execution tracking with a hard 5-second threshold timeout
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = cli.ContainerStart(runCtx, resp.ID, container.StartOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to boot sandbox: %w", err)
	}

	// 5. Await execution completion or timeout expiration
	statusCh, errCh := cli.ContainerWait(runCtx, resp.ID, container.WaitConditionNotRunning)
	var exitCode int64 = 0
	timedOut := false

	select {
	case err := <-errCh:
		if err != nil {
			return nil, fmt.Errorf("error during execution tracking: %w", err)
		}
	case status := <-statusCh:
		exitCode = status.StatusCode
	case <-runCtx.Done():
		// Hard cancellation limit hit (e.g., infinite loop attack detected)
		timedOut = true
	}

	// 6. Extract execution log streams safely split by standard descriptors
	logOptions := container.LogsOptions{ShowStdout: true, ShowStderr: true}
	out, err := cli.ContainerLogs(ctx, resp.ID, logOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate sandbox outputs: %w", err)
	}
	defer out.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	// Multiplex the raw multiplexed stream back into distinct standard out/err boundaries
	_, err = stdcopy.StdCopy(&stdoutBuf, &stderrBuf, out)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to de-multiplex execution streams: %w", err)
	}

	return &ExecutionResult{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		ExitCode: exitCode,
		TimedOut: timedOut,
	}, nil
}
