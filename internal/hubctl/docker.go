package hubctl

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Roman-Samoilenko/privacy-hub/internal/config"
	"github.com/Roman-Samoilenko/privacy-hub/internal/logger"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type DockerManager struct {
	cli *client.Client
	cfg config.DockerContainerConfig
}

func NewDockerManager(cfg config.DockerContainerConfig) (*DockerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %v", err)
	}

	return &DockerManager{
		cli: cli,
		cfg: cfg,
	}, nil
}

func (dm *DockerManager) Start(ctx context.Context) error {
	// Check if container already exists
	containerID, err := dm.findContainer(ctx)
	if err != nil {
		return err
	}

	if containerID != "" {
		// Container exists, just start it
		logger.Infof("Starting existing container: %s", dm.cfg.Name)
		return dm.cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
	}

	// Create new container
	logger.Infof("Creating new container: %s", dm.cfg.Name)
	return dm.createAndStart(ctx)
}

func (dm *DockerManager) createAndStart(ctx context.Context) error {
	portStr := fmt.Sprintf("%d/udp", dm.cfg.Listen)
	tcpPortStr := fmt.Sprintf("%d/tcp", dm.cfg.Listen+1)

	containerCfg := &container.Config{
		Image: dm.cfg.Image,
		ExposedPorts: nat.PortSet{
			nat.Port(portStr):    struct{}{},
			nat.Port(tcpPortStr): struct{}{},
		},
		Env: []string{
			"LOG_LEVEL=info",
		},
	}

	hostCfg := &container.HostConfig{
		PortBindings: nat.PortMap{
			nat.Port(portStr): []nat.PortBinding{
				{
					HostIP:   "127.0.0.1",
					HostPort: fmt.Sprintf("%d", dm.cfg.Listen),
				},
			},
			nat.Port(tcpPortStr): []nat.PortBinding{
				{
					HostIP:   "127.0.0.1",
					HostPort: fmt.Sprintf("%d", dm.cfg.Listen+1),
				},
			},
		},
		RestartPolicy: container.RestartPolicy{
			Name: dm.cfg.RestartPolicy,
		},
		NetworkMode: container.NetworkMode(dm.cfg.Network),
	}

	resp, err := dm.cli.ContainerCreate(
		ctx,
		containerCfg,
		hostCfg,
		nil,
		nil,
		dm.cfg.Name,
	)
	if err != nil {
		return fmt.Errorf("failed to create container: %v", err)
	}

	logger.Successf("Created container: %s", resp.ID[:12])

	if err := dm.cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %v", err)
	}

	logger.Successf("Started container: %s", dm.cfg.Name)
	return nil
}

func (dm *DockerManager) Stop(ctx context.Context) error {
	containerID, err := dm.findContainer(ctx)
	if err != nil {
		return err
	}

	if containerID == "" {
		logger.Warnf("Container %s not found", dm.cfg.Name)
		return nil
	}

	logger.Infof("Stopping container: %s", dm.cfg.Name)

	timeout := 10
	stopOptions := container.StopOptions{
		Timeout: &timeout,
	}

	if err := dm.cli.ContainerStop(ctx, containerID, stopOptions); err != nil {
		return fmt.Errorf("failed to stop container: %v", err)
	}

	logger.Successf("Stopped container: %s", dm.cfg.Name)
	return nil
}

func (dm *DockerManager) Remove(ctx context.Context) error {
	containerID, err := dm.findContainer(ctx)
	if err != nil {
		return err
	}

	if containerID == "" {
		return nil
	}

	logger.Infof("Removing container: %s", dm.cfg.Name)

	if err := dm.cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
		Force: true,
	}); err != nil {
		return fmt.Errorf("failed to remove container: %v", err)
	}

	logger.Successf("Removed container: %s", dm.cfg.Name)
	return nil
}

func (dm *DockerManager) findContainer(ctx context.Context) (string, error) {
	containers, err := dm.cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %v", err)
	}

	for _, c := range containers {
		for _, name := range c.Names {
			if name == "/"+dm.cfg.Name || name == dm.cfg.Name {
				return c.ID, nil
			}
		}
	}

	return "", nil
}

func (dm *DockerManager) WaitReady(ctx context.Context, timeout time.Duration) error {
	logger.Infof("Waiting for container to be ready...")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for container")
		case <-ticker.C:
			containerID, err := dm.findContainer(ctx)
			if err != nil {
				return err
			}

			if containerID == "" {
				continue
			}

			inspect, err := dm.cli.ContainerInspect(ctx, containerID)
			if err != nil {
				return err
			}

			if inspect.State.Running {
				logger.Successf("Container is ready")
				return nil
			}
		}
	}
}

func (dm *DockerManager) Logs(ctx context.Context, follow bool) (io.ReadCloser, error) {
	containerID, err := dm.findContainer(ctx)
	if err != nil {
		return nil, err
	}

	if containerID == "" {
		return nil, fmt.Errorf("container not found")
	}

	return dm.cli.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: true,
	})
}

func (dm *DockerManager) Close() error {
	if dm.cli != nil {
		return dm.cli.Close()
	}
	return nil
}
