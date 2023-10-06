package svc

import "github.com/peergoim/signaling-server/internal/config"

type ServiceContext struct {
	Config *config.Config
}

func NewServiceContext(c *config.Config) *ServiceContext {
	s := &ServiceContext{Config: c}
	return s
}
