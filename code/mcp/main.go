package main

import (
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/mcp/config"
	"go.uber.org/fx"
)

func main() {
	fx.New(
		config.Module,
		fx.Provide(newMCPHTTPHandler),
		fx.Invoke(startHTTPServer),
	).Run()
}
