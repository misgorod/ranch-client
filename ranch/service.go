package ranch

import (
	"context"
	docker "github.com/docker/docker/client"
	"github.com/pkg/errors"
	"github.com/ybbus/jsonrpc"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"ranchclient/container"
	"sync"
	"time"
)

type Service struct {
	id        int
	docker    *docker.Client
	rpc       jsonrpc.RPCClient
	username  string
	password  string
	container *container.Container
	mutex     *sync.Mutex
}

func NewService(master string) (*Service, error) {
	dockerClient, err := docker.NewEnvClient()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create docker client")
	}
	rpcClient := jsonrpc.NewClient(master)
	s := &Service{
		docker: dockerClient,
		rpc: rpcClient,
		mutex: &sync.Mutex{},
	}
	return s, nil
}

type RegisterRequest struct {
	Id int `json:"id"`
}

func (s *Service) Register(id int, username, password string) error {
	s.id = id
	s.username = username
	s.password = password
	_, err := s.rpc.Call("Ranch.Register", &RegisterRequest{id})
	if err != nil {
		switch e := err.(type) {
		default:
			return errors.Wrap(e, "Failed to register client dur to RPC error")
		case *jsonrpc.HTTPError:
			return errors.Wrap(e, "Failed to register client due to HTTP error")
		}
	}
	log.Println("Registered client")
	return nil
}

type CreateRequest struct {
	Image string `json:"image"`
	ConfPath string `json:"confPath"`
}

type CreateResponse struct {
	Port int `json:"port"`
}

func (s *Service) Create(r *http.Request, args *CreateRequest, reply *CreateResponse) error {
	ctx := r.Context()
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.container == nil {
		hostDir, err := ioutil.TempDir("", "ranch")
		if err != nil {
			return errors.Wrap(err, "Failed to create temporary dir for container")
		}
		log.Printf("Created temp directory in %v\n", hostDir)
		container, err := container.NewContainer(ctx, s.docker, s.id, args.Image, args.ConfPath, hostDir, s.username, s.password)
		if err != nil {
			_ = os.RemoveAll(hostDir)
			return errors.Wrap(err, "Failed to create container")
		}
		log.Println("Container created successfully")
		s.container = container
	} else {
		return errors.New("Container already created")
	}
	reply.Port = s.container.HostPort()
	return nil
}

type StartRequest struct {
	Config map[string]string
}

type StartResponse struct {}

func (s *Service) Start(r *http.Request, args *StartRequest, reply *StartResponse) error {
	ctx := r.Context()
	dir := s.container.Dir()
	err := createConfig(args.Config, dir)
	if err != nil {
		return errors.Wrap(err, "Failed to create config")
	}
	running, err := s.container.Running(ctx, s.docker)
	if err != nil {
		return errors.Wrap(err, "Failed to check container status")
	}
	if running {
		err = s.container.Restart(ctx, s.docker)
		if err != nil {
			return errors.Wrap(err, "Failed to restart container")
		}
	} else {
		err = s.container.Start(ctx, s.docker)
		if err != nil {
			return errors.Wrap(err, "Failed to start container")
		}
	}
	go func(s *Service) {
		time.Sleep(time.Second * 2)
		logs, err := s.container.Logs(context.Background(), s.docker)
		if err != nil {
			log.Printf("Failed to get logs from container with name %v: ", s.container.Name())
			return
		}
		log.Printf("Logs from container with name %v: %s", s.container.Name(), logs)
	}(s)
	return nil
}

func createConfig(config map[string]string, path string) error {
	data := []byte(parseConfig(config))
	fpath := filepath.Join(path, "server.properties")
	log.Printf("Wrote config data to %v\n", fpath)
	err := ioutil.WriteFile(fpath, data, 0644)
	if err != nil {
		return errors.Wrap(err, "Failed to write config")
	}
	return nil
}

func parseConfig(config map[string]string) string {
	var result string
	for k, v := range config {
		result += k + "=" + v + "\n"
	}
	return result
}

func (s *Service) Clean(ctx context.Context) {
	_ = os.RemoveAll(s.container.Dir())
	err := s.container.Remove(ctx, s.docker)
	if err != nil {
		log.Println("Failed to remove container %v", s.container.Name())
	}
}
