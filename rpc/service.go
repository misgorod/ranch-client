package ranch

import (
	docker "github.com/docker/docker/client"
	"github.com/pkg/errors"
	"net/http"
	"ranch-client/container"
)

type Service struct {
	id int
	client *docker.Client
	username string
	password string
	container *container.Container
}

func NewService(id int, username, password string) (*Service, error) {
	client, err := docker.NewEnvClient()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create docker client")
	}
	s := &Service{
		id,
		client,
		username,
		password,
		nil,
	}
	return s, nil
}

type StartArgs struct {}

type StartReply struct {}

func (s *Service) Start(r *http.Request, args *StartArgs, reply *StartReply) error {

	return nil
}
