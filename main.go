package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/gorilla/rpc/v2"
	json "github.com/gorilla/rpc/v2/json2"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"ranchclient/ranch"
	"syscall"
	"time"
)

var (
	master   = flag.String("master", "", "REQUIRED: Address of master server")
	clientID = flag.Int("id", -1, "REQUIRED: Id of client. Must be unique and more than 0")
	port     = flag.Uint("port", 8080, "Port to listen on")
	username = flag.String("docker-user", "", "REQUIRED: Login on dockerhub for image pulling")
	password = flag.String("docker-pass", "", "REQUIRED: Password or token on dockerhub for image pulling")
)

func main() {
	validateArgs()
	service, err := ranch.NewService(*master)
	if err != nil {
		log.Fatalf("Failed to create service: %+v", err)
	}
	err = service.Register(*clientID, *username, *password)
	if err != nil {
		log.Fatalf("Failed to register client: %+v", err)
	}

	rpcServer := rpc.NewServer()
	rpcServer.RegisterCodec(json.NewCodec(), "application/json")
	rpcServer.RegisterService(service, "Client")

	listenAddr := fmt.Sprintf(":%d", *port)
	serveMux := http.DefaultServeMux
	serveMux.Handle("/rpc", rpcServer)
	server := &http.Server{
		Addr:              listenAddr,
		Handler:           serveMux,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGQUIT, syscall.SIGSTOP, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)
	go gracefulShutdown(quit, server, service)

	log.Printf("Listening on %d\n", *port)
	log.Fatalln(server.ListenAndServe())
}

func validateArgs() {
	flag.Parse()
	if *master == "" || !IsUrl(*master) {
		log.Fatalf("Invalid master address: %s\n", *master)
	}
	if *clientID < 1 {
		log.Fatalf("Invalid client id: %d\n", *clientID)
	}
	if *username == "" {
		log.Fatalln("Username must be provided")
	}
	if *password == "" {
		log.Fatalln("Password must be provided")
	}
}


func IsUrl(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

func gracefulShutdown(quit <-chan os.Signal, server *http.Server, service *ranch.Service) {
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	service.Clean(ctx)
	err := server.Shutdown(ctx)
	if err != nil {
		log.Println("Failed to shutdown gracefully due to HTTP server")
		return
	}
	log.Println("Service was shutdown gracefully")
}