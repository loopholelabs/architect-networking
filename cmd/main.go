package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/loopholelabs/cmdutils/pkg/command"
	"github.com/loopholelabs/cmdutils/pkg/version"

	"github.com/loopholelabs/architect-networking/cmd/failover"
	"github.com/loopholelabs/architect-networking/internal/config"
	architectVersion "github.com/loopholelabs/architect-networking/version"
)

var cmd = command.New(
	"arc-net",
	"Architect for Networking",
	"Architect for Networking - Architect's high-performance network data plane.",
	true,
	version.New[*config.Config](architectVersion.GitCommit, architectVersion.GoVersion, architectVersion.Platform, architectVersion.Version, architectVersion.BuildDate),
	config.New,
	[]command.SetupCommand[*config.Config]{
		failover.Cmd(),
	},
)

func main() {
	os.Exit(realMain())
}

func realMain() int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	return cmd.Execute(ctx, command.Interactive)
}
