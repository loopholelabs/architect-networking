package failover

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/loopholelabs/goroutine-manager/pkg/manager"
	"github.com/loopholelabs/logging/types"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	"github.com/loopholelabs/architect-networking/pkg/client"
)

var (
	ErrPrimarySocketRequired      = errors.New("primary socket address is required")
	ErrSecondarySocketRequired    = errors.New("secondary socket address is required")
	ErrCreatingPrimaryClient      = errors.New("failed to create primary client")
	ErrCreatingSecondaryClient    = errors.New("failed to create secondary client")
	ErrCreatingPrimaryAPIClient   = errors.New("failed to create primary API client")
	ErrCreatingSecondaryAPIClient = errors.New("failed to create secondary API client")
	ErrCreatingGoroutineManager   = errors.New("failed to create goroutine manager")
	ErrParsingMultiaddr           = errors.New("failed to parse multiaddr")
	ErrDialingSocket              = errors.New("failed to dial socket")
	ErrGettingStateFromPrimary    = errors.New("failed to get state from primary")
	ErrPrimaryServerStatus        = errors.New("primary server returned error status")
	ErrPrimaryServerEmptyState    = errors.New("primary server returned empty state")
	ErrSettingStateOnSecondary    = errors.New("failed to set state on secondary")
	ErrSecondaryServerStatus      = errors.New("secondary server returned error status")
)

type Config struct {
	PrimarySocket   string        `yaml:"primary_socket"   mapstructure:"primary_socket"`
	SecondarySocket string        `yaml:"secondary_socket" mapstructure:"secondary_socket"`
	Interval        time.Duration `yaml:"interval"         mapstructure:"interval"`
	Logger          types.Logger
}

func (c *Config) Validate() error {
	if c.PrimarySocket == "" {
		return ErrPrimarySocketRequired
	}
	if c.SecondarySocket == "" {
		return ErrSecondarySocketRequired
	}
	if c.Interval <= 0 {
		c.Interval = 30 * time.Second
	}
	return nil
}

type Failover struct {
	config           *Config
	logger           types.Logger
	goroutineManager *manager.GoroutineManager
	primaryClient    *client.ClientWithResponses
	secondaryClient  *client.ClientWithResponses
}

func New(config *Config) (*Failover, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	logger := config.Logger

	// Create HTTP clients for unix socket connections using multiaddr net
	primaryClient, err := createUnixSocketClient(config.PrimarySocket)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreatingPrimaryClient, err)
	}

	secondaryClient, err := createUnixSocketClient(config.SecondarySocket)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreatingSecondaryClient, err)
	}

	// Create OpenAPI clients
	primaryAPIClient, err := client.NewClientWithResponses("http://localhost", client.WithHTTPClient(primaryClient))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreatingPrimaryAPIClient, err)
	}

	secondaryAPIClient, err := client.NewClientWithResponses(
		"http://localhost",
		client.WithHTTPClient(secondaryClient),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreatingSecondaryAPIClient, err)
	}

	return &Failover{
		config:          config,
		logger:          logger,
		primaryClient:   primaryAPIClient,
		secondaryClient: secondaryAPIClient,
	}, nil
}

func (f *Failover) Start(ctx context.Context) error {
	var errs error

	f.goroutineManager = manager.NewGoroutineManager(
		ctx,
		&errs,
		manager.GoroutineManagerHooks{},
	)
	if errs != nil {
		return fmt.Errorf("%w: %w", ErrCreatingGoroutineManager, errs)
	}

	f.logger.Info().
		Str("primary", f.config.PrimarySocket).
		Str("secondary", f.config.SecondarySocket).
		Str("interval", f.config.Interval.String()).
		Msg("Failover started")

	// Start the sync goroutine
	f.goroutineManager.StartBackgroundGoroutine(func(ctx context.Context) {
		ticker := time.NewTicker(f.config.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := f.syncNATState(ctx); err != nil {
					f.logger.Error().Err(err).Msg("Failed to sync NAT state")
				} else {
					f.logger.Debug().Msg("NAT state synced successfully")
				}
			}
		}
	})

	<-f.goroutineManager.Context().Done()
	return nil
}

func (f *Failover) Stop() error {
	if f.goroutineManager != nil {
		f.logger.Info().Msg("Failover shutting down")
		f.goroutineManager.StopAllGoroutines()
		f.goroutineManager.Wait()
	}
	return nil
}

func createUnixSocketClient(socketAddr string) (*http.Client, error) {
	// Parse the multiaddr
	maddr, err := multiaddr.NewMultiaddr(socketAddr)
	if err != nil {
		return nil, fmt.Errorf("%w %s: %w", ErrParsingMultiaddr, socketAddr, err)
	}

	// Create HTTP client with custom transport for unix socket
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				// Use multiaddr net to dial
				conn, err := manet.Dial(maddr)
				if err != nil {
					return nil, fmt.Errorf("%w %s: %w", ErrDialingSocket, socketAddr, err)
				}
				return conn, nil
			},
		},
		Timeout: 30 * time.Second,
	}, nil
}

func (f *Failover) syncNATState(ctx context.Context) error {
	// Export state from primary
	resp, err := f.primaryClient.GetStateWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrGettingStateFromPrimary, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("%w %d: %s", ErrPrimaryServerStatus, resp.StatusCode(), string(resp.Body))
	}

	if resp.JSON200 == nil {
		return ErrPrimaryServerEmptyState
	}

	natState := *resp.JSON200

	// Import state to secondary
	importResp, err := f.secondaryClient.SetStateWithResponse(ctx, natState)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSettingStateOnSecondary, err)
	}

	if importResp.StatusCode() != http.StatusOK {
		return fmt.Errorf("%w %d: %s", ErrSecondaryServerStatus, importResp.StatusCode(), string(importResp.Body))
	}

	return nil
}
