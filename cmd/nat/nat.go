package nat

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	manet "github.com/multiformats/go-multiaddr/net"

	"github.com/spf13/cobra"

	"github.com/loopholelabs/cmdutils"
	"github.com/loopholelabs/cmdutils/pkg/command"
	"github.com/loopholelabs/goroutine-manager/pkg/manager"

	"github.com/loopholelabs/conduit/pkg/emitter"
	"github.com/loopholelabs/conduit/pkg/server"
	"github.com/loopholelabs/conduit/pkg/statsd"
	"github.com/loopholelabs/conduit/pkg/transit"

	"github.com/loopholelabs/architect-networking/internal/config"
)

func Cmd() command.SetupCommand[*config.Config] {
	return func(cmd *cobra.Command, ch *cmdutils.Helper[*config.Config]) {
		c := &cobra.Command{
			Use:   "nat",
			Short: "Architect NAT",
			PreRunE: func(_ *cobra.Command, _ []string) error {
				if err := ch.Config.Server.Parse(); err != nil {
					ch.Logger.Error().Err(err).Msg("Failed to parse server config")
					return err
				}
				if err := ch.Config.Server.Validate(); err != nil {
					ch.Logger.Error().Err(err).Msg("Failed to validate server config")
					return err
				}
				return nil
			},
			RunE: func(_ *cobra.Command, _ []string) error {
				ch.Printer.Printf("Running Architect NAT...")

				return run(ch)
			},
		}

		cmd.AddCommand(c)
	}
}

func run(ch *cmdutils.Helper[*config.Config]) error {
	var errs error
	logger := ch.Logger.SubLogger("NAT")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	goroutineManager := manager.NewGoroutineManager(
		ctx,
		&errs,
		manager.GoroutineManagerHooks{},
	)
	if errs != nil {
		return errors.Join(errors.New("failed to create goroutine manager"), errs)
	}
	defer goroutineManager.Wait()
	defer goroutineManager.StopAllGoroutines()

	go func() {
		done := make(chan os.Signal, 1)
		signal.Notify(done, os.Interrupt)

		<-done

		logger.Info().Msg("Exiting gracefully")

		goroutineManager.StopAllGoroutines()
	}()

	var em emitter.Emitter
	if ch.Config.StatsD != nil {
		statsdClient := statsd.NewClient(ch.Config.StatsD)
		em = statsdClient.Emitter
		defer func() {
			_ = statsdClient.Close()
		}()
	} else {
		em = emitter.NewNoopEmitter()
	}
	transitLogger := ch.Logger.SubLogger("transit")
	tr, err := transit.New(transitLogger, ch.Config.Transit, em)
	if err != nil {
		logger.Error().Err(err).Msg("failed to create transit")
		panic(errors.Join(errors.New("failed to create transit"), err))
	}
	defer func() {
		_ = tr.Close()
	}()

	serverLogger := ch.Logger.SubLogger("server")
	serverConfig := ch.Config.Server
	serverConfig.Logger = serverLogger
	s, err := server.NewServer(serverConfig, tr)
	if err != nil {
		logger.Error().Err(err).Msg("failed to create server")
		panic(errors.Join(errors.New("failed to create server"), err))
	}

	defer func() {
		defer goroutineManager.CreateForegroundPanicCollector()()

		if err := s.Shutdown(); err != nil {
			panic(errors.Join(errors.New("failed to shutdown server"), err))
		}
	}()
	lis, err := manet.Listen(serverConfig.HTTPAddr)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = lis.Close() // This will automatically clean up unix socket files
	}()

	m := http.NewServeMux()
	m.Handle("/v1/", http.StripPrefix("/v1", s.HTTPHandler()))

	httpServer := &http.Server{
		ReadHeaderTimeout: 2 * time.Second,
		BaseContext: func(_ net.Listener) context.Context {
			return goroutineManager.Context()
		},
		Handler: m,
	}
	defer func() {
		defer goroutineManager.CreateForegroundPanicCollector()()

		if err := httpServer.Shutdown(context.Background()); err != nil {
			panic(errors.Join(errors.New("failed to shutdown HTTP server"), err))
		}
	}()

	logger.Info().Str("addr", lis.Multiaddr().String()).Msg("HTTP API listening")

	goroutineManager.StartForegroundGoroutine(func(_ context.Context) {
		logger.Info().Msg("starting HTTP server...")
		if err := httpServer.Serve(manet.NetListener(lis)); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Info().Err(err).Msg("HTTP server error")
			panic(err)
		}
		logger.Info().Msg("HTTP server stopped")
	})

	<-goroutineManager.Context().Done()
	return nil
}
