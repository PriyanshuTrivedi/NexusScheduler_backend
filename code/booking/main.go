package main

import (
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/config"
	"go.uber.org/fx"
)

func main() {
	fx.New(config.Module).Run()
}
