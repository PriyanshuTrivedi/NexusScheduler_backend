package main

import (
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/api-gateway/config"
	"go.uber.org/fx"
)

func main() { fx.New(config.Module).Run() }
