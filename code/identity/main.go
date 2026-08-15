package main

import (
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/config"
	"go.uber.org/fx"
)

func main() { fx.New(config.Module).Run() }
