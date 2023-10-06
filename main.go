package main

import (
	"flag"
	"fmt"
	"github.com/peergoim/signaling-server/internal/config"
	"github.com/peergoim/signaling-server/internal/server"
	"github.com/peergoim/signaling-server/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
)

var configPath = flag.String("f", "etc/config.yaml", "config file path")

func main() {
	flag.Parse()
	c := &config.Config{}
	conf.MustLoad(*configPath, c)
	err := c.Validate()
	if err != nil {
		panic(fmt.Errorf("validate config file: %s \n", err))
	}
	ctx := svc.NewServiceContext(c)
	server.NewWebSocketServer(ctx).Start()
}
