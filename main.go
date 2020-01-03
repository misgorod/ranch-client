package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
	"github.com/ybbus/jsonrpc"
	"log"
	"net/http"
	"net/url"
	ranch "ranch-client/rpc"
)

var (
	master = flag.String("master", "", "REQUIRED: Address of master server")
	clientID = flag.Int("id", -1, "REQUIRED: Id of client. Must be unique and more than 0")
	port = flag.Uint("port", 8080, "Port to listen on")
	username = flag.String("docker-user", "", "REQUIRED: Login on dockerhub for image pulling")
	password = flag.String("docker-pass", "", "REQUIRED: Password or token on dockerhub for image pulling")
)

type RegisterRequest struct {
	Id int `json:"id"`
}

func main() {
	flag.Parse()
	if *master == "" || !IsUrl(*master) {
		log.Fatalf("Invalid master address: %s\n", *master)
	}
	if *clientID < 1 {
		log.Fatalf("Invalid client id: %d\n", *clientID)
	}

	client := jsonrpc.NewClient(*master)
	_, err := client.Call("Ranch.Register", &RegisterRequest{*clientID})
	if err != nil {
		switch e := err.(type) {
		case *jsonrpc.HTTPError:
			log.Fatalf("Failed to register client due to HTTP error: code: %d error: %s\n", e.Code, e.Error())
		default:
			log.Fatalf("Failed to register client dur to RPC error: %s\n", e.Error())
		}
	}

	service, err := ranch.NewService(*clientID, *username, *password)
	if err != nil {
		log.Fatalf("Failed to create service: %+v", err)
	}

	server := rpc.NewServer()
	server.RegisterCodec(json2.NewCodec(), "application/json")
	err = server.RegisterService(service, "Client")
	if err != nil {
		log.Fatalln("Failed to register rpc service")
	}
	http.Handle("/rpc", server)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}

func IsUrl(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}
