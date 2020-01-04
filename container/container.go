package container

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	docker "github.com/docker/docker/client"
	"github.com/phayes/freeport"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"time"
)

const (
	kafkaPort = "9092"
)

type Container struct {
	id       string
	name     string
	dir      string
	hostPort int
}

func (c *Container) HostPort() int {
	return c.hostPort
}

func (c *Container) Name() string {
	return c.name
}

func (c *Container) Dir() string {
	return c.dir
}

func NewContainer(ctx context.Context, client *docker.Client, id int, image, confPath, hostDir, username, password string) (*Container, error) {
	authStr, err := authEncode(username, password)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to encode auth data")
	}
	_, err = client.ImagePull(ctx, image, types.ImagePullOptions{RegistryAuth: authStr})
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to pull image %s", image)
	}
	containerName := fmt.Sprintf("ranch-%d", id)
	//containerPort, err := nat.NewPort("tcp", kafkaPort)
	//if err != nil {
	//	return nil, errors.Wrap(err, "Failed to create port binding")
	//}
	hostPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get free port")
	}
	config := container.Config{
		//ExposedPorts:    nat.PortSet{containerPort: struct{}{}},
		Tty:             true,
		Image:           image,
		Volumes:         nil,
		NetworkDisabled: false,
	}
	hostConfig := container.HostConfig{
		NetworkMode: "host",
		//PortBindings: nat.PortMap{kafkaPort: []nat.PortBinding{
		//	{
		//		HostIP:   "0.0.0.0",
		//		HostPort: strconv.Itoa(hostPort),
		//	},
		//}},
		AutoRemove: false,
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: hostDir,
				Target: confPath,
			},
		},
	}
	body, err := client.ContainerCreate(ctx, &config, &hostConfig, nil, containerName)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create container %s", containerName)
	}
	c := &Container{
		body.ID,
		containerName,
		hostDir,
		hostPort,
	}
	return c, nil
}

func (c *Container) Start(ctx context.Context, client *docker.Client) error {
	err := client.ContainerStart(ctx, c.id, types.ContainerStartOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to start container")
	}
	return nil
}

func (c *Container) Logs(ctx context.Context, client *docker.Client) (string, error) {
	rc, err := client.ContainerLogs(ctx, c.id, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Follow:     false,
	})
	if err != nil {
		return "", errors.Wrap(err, "Failed to get container logs")
	}
	logs, err := ioutil.ReadAll(rc)
	if err != nil {
		return "", errors.Wrap(err, "Failed to read container logs")
	}
	err = rc.Close()
	if err != nil {
		return "", errors.Wrap(err, "Failed to read container logs")
	}
	return string(logs), nil
}

func (c *Container) Restart(ctx context.Context, client *docker.Client) error {
	timeout := time.Second * 5
	err := client.ContainerRestart(ctx, c.id, &timeout)
	if err != nil {
		return errors.Wrap(err, "Failed to restart container")
	}
	return nil
}

func (c *Container) Running(ctx context.Context, client *docker.Client) (bool, error) {
	stat, err := client.ContainerInspect(ctx, c.id)
	if err != nil {
		return false, errors.Wrap(err, "Failed to get state of container")
	}
	return stat.State.Running, nil
}

func (c *Container) Remove(ctx context.Context, client *docker.Client) error {
	err := os.RemoveAll(c.dir)
	if err != nil {
		return errors.Wrap(err, "Failed to remove temp container dir")
	}
	err = client.ContainerRemove(ctx, c.id, types.ContainerRemoveOptions{
		Force: true,
	})
	if err != nil {
		return errors.Wrapf(err, "Failed to remove container with id %s", c.id)
	}
	return nil
}

func authEncode(username, password string) (string, error) {
	authConfig := types.AuthConfig{
		Username: username,
		Password: password,
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return "", errors.Wrap(err, "Failed to marshal auth config")
	}
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)
	return authStr, nil
}
