package controllers

import (
	server "github.com/k3s-io/k3s/pkg/server"
)

type Server interface {
	LeaderControllers() server.CustomControllers
	Controllers() server.CustomControllers
}
