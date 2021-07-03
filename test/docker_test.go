package main

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

var isDarwin = false

func startContainer(cfg *container.Config, hostCfg *container.HostConfig, name string) (string, error) {
	c, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return "", err
	}
	defer c.Close()

	if !isDarwin {
		hostCfg.NetworkMode = "host"
	}

	container, err := c.ContainerCreate(context.Background(), cfg, hostCfg, nil, nil, name)
	if err != nil {
		return "", err
	}

	if err = c.ContainerStart(context.Background(), container.ID, types.ContainerStartOptions{}); err != nil {
		return "", err
	}

	return container.ID, nil
}

func cleanContainer(id string) error {
	c, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer c.Close()

	removeOpts := types.ContainerRemoveOptions{Force: true}
	return c.ContainerRemove(context.Background(), id, removeOpts)
}
