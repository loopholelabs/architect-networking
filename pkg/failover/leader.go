package failover

//go:generate protoc --go-frpc_out=. failover.proto --go-frpc_opt=paths=source_relative

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/loopholelabs/logging/types"

	"github.com/loopholelabs/architect-networking/pkg/client"
)

// NodeRole represents the current role of the failover node
type NodeRole int

const (
	RoleUnknown NodeRole = iota
	RolePrimary
	RoleSecondary
)

// Role string constants
const (
	RoleStringPrimary   = "primary"
	RoleStringSecondary = "secondary"
	RoleStringUnknown   = "unknown"
)

func (r NodeRole) String() string {
	switch r {
	case RolePrimary:
		return RoleStringPrimary
	case RoleSecondary:
		return RoleStringSecondary
	default:
		return RoleStringUnknown
	}
}

// portUint32ToUint16 converts uint32 port to uint16, clamping to valid port range
func portUint32ToUint16(port uint32) uint16 {
	if port > 65535 {
		return 65535
	}
	return uint16(port)
}

// LeaderConfig extends the basic failover config with leader election parameters
type LeaderConfig struct {
	// ENI IP address to monitor for ownership
	ENIIP string `yaml:"eni_ip" mapstructure:"eni_ip"`

	// Port for frpc communication between nodes
	Port uint16 `yaml:"port" mapstructure:"port"`

	// Interval for checking ENI ownership
	LeaderCheckInterval time.Duration `yaml:"leader_check_interval" mapstructure:"leader_check_interval"`

	// Interval for syncing NAT state (when acting as secondary)
	SyncInterval time.Duration `yaml:"sync_interval" mapstructure:"sync_interval"`

	// Heartbeat interval (must be <50ms for 3 heartbeats in 150ms)
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval" mapstructure:"heartbeat_interval"`

	// Number of missed heartbeats before failover
	HeartbeatMissThreshold int `yaml:"heartbeat_miss_threshold" mapstructure:"heartbeat_miss_threshold"`

	// Local conduit server socket for API access
	LocalSocket string `yaml:"local_socket" mapstructure:"local_socket"`

	// Destination CIDR block for route table updates
	DestinationCIDR string `yaml:"destination_cidr" mapstructure:"destination_cidr"`

	// Disable ENI ownership checks for testing purposes
	DisableENICheck bool `yaml:"disable_eni_check" mapstructure:"disable_eni_check"`

	// Force role for testing (primary or secondary)
	ForceRole string `yaml:"force_role" mapstructure:"force_role"`

	// Logger instance
	Logger types.Logger
}

func (c *LeaderConfig) Validate() error {
	if c.ENIIP == "" {
		return errors.New("ENI IP address is required")
	}
	if !c.DisableENICheck {
		if net.ParseIP(c.ENIIP) == nil {
			return fmt.Errorf("invalid ENI IP address: %s", c.ENIIP)
		}
	}
	if c.Port == 0 {
		c.Port = 1022 // Default port
	}
	if c.LocalSocket == "" {
		return errors.New("local socket address is required")
	}
	if c.LeaderCheckInterval <= 0 {
		c.LeaderCheckInterval = 30 * time.Second
	}
	if c.SyncInterval <= 0 {
		c.SyncInterval = 10 * time.Second
	}
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = 40 * time.Millisecond // Default 40ms to allow 3 heartbeats in <150ms
	}
	if c.HeartbeatMissThreshold <= 0 {
		c.HeartbeatMissThreshold = 3 // Default to 3 missed heartbeats
	}
	if c.ForceRole != "" && c.ForceRole != RoleStringPrimary && c.ForceRole != RoleStringSecondary {
		return fmt.Errorf("force-role must be 'primary' or 'secondary', got: %s", c.ForceRole)
	}
	return nil
}

// LeaderFailover implements leader election based failover using AWS ENI ownership
type LeaderFailover struct {
	config      *LeaderConfig
	logger      types.Logger
	awsClient   *AWSClient
	localClient *client.ClientWithResponses

	// Current role and state
	currentRole NodeRole
	currentENI  string // ENI ID of current primary

	// fRPC server (when acting as primary)
	frpcServer *Server

	// fRPC client (when acting as secondary)
	frpcClient *Client

	// Heartbeat tracking
	lastHeartbeat    time.Time
	missedHeartbeats int
	heartbeatMutex   sync.Mutex
	heartbeatStopCh  chan struct{}

	// Control channels
	stopCh chan struct{}
	roleCh chan NodeRole
}

// NewLeaderFailover creates a new leader election based failover instance
func NewLeaderFailover(config *LeaderConfig) (*LeaderFailover, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	logger := config.Logger

	// Create AWS client for ENI ownership detection (if not disabled)
	var awsClient *AWSClient
	if !config.DisableENICheck {
		ctx := context.Background()
		var err error
		awsClient, err = NewAWSClient(ctx, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create AWS client: %w", err)
		}
	}

	// Create local client for conduit API access
	localClient, err := createUnixSocketClient(config.LocalSocket)
	if err != nil {
		return nil, fmt.Errorf("failed to create local client: %w", err)
	}

	localAPIClient, err := client.NewClientWithResponses("http://localhost", client.WithHTTPClient(localClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create local API client: %w", err)
	}

	return &LeaderFailover{
		config:      config,
		logger:      logger,
		awsClient:   awsClient,
		localClient: localAPIClient,
		currentRole: RoleUnknown,
		stopCh:      make(chan struct{}),
		roleCh:      make(chan NodeRole, 1),
	}, nil
}

// Start begins the leader election and failover process
func (lf *LeaderFailover) Start(ctx context.Context) error {
	logEvent := lf.logger.Info().
		Str("eni_ip", lf.config.ENIIP).
		Uint16("port", lf.config.Port)

	if lf.awsClient != nil {
		logEvent = logEvent.Str("instance_id", lf.awsClient.GetInstanceID())
	} else {
		logEvent = logEvent.Str("instance_id", "test-mode")
	}

	logEvent.Msg("Starting leader election failover")

	lf.logger.Info().
		Str("eni_ip", lf.config.ENIIP).
		Uint16("port", lf.config.Port).
		Str("leader_check_interval", lf.config.LeaderCheckInterval.String()).
		Bool("disable_eni_check", lf.config.DisableENICheck).
		Str("force_role", lf.config.ForceRole).
		Msg("Leader election configuration")

	// Start the leader election loop
	lf.logger.Debug().Msg("Starting leader election loop")
	go lf.leaderElectionLoop(ctx)

	// Start the role management loop
	lf.logger.Debug().Msg("Starting role management loop")
	go lf.roleManagementLoop(ctx)

	// Wait for context cancellation
	<-ctx.Done()
	close(lf.stopCh)

	return lf.cleanup()
}

// Stop gracefully shuts down the failover system
func (lf *LeaderFailover) Stop() error {
	if lf.stopCh != nil {
		close(lf.stopCh)
	}
	return lf.cleanup()
}

// GetCurrentRole returns the current role of this node
func (lf *LeaderFailover) GetCurrentRole() NodeRole {
	return lf.currentRole
}

// leaderElectionLoop continuously checks ENI ownership to determine leadership
func (lf *LeaderFailover) leaderElectionLoop(ctx context.Context) {
	ticker := time.NewTicker(lf.config.LeaderCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-lf.stopCh:
			return
		case <-ticker.C:
			lf.logger.Debug().
				Str("current_role", string(rune(lf.currentRole))).
				Str("eni_ip", lf.config.ENIIP).
				Bool("disable_eni_check", lf.config.DisableENICheck).
				Str("force_role", lf.config.ForceRole).
				Msg("Starting leader election check")

			// Skip ENI checks if we're secondary - heartbeat monitoring takes precedence
			if lf.currentRole == RoleSecondary {
				lf.logger.Debug().Msg("Skipping ENI check - currently secondary, heartbeat monitoring active")
				continue
			}

			var newRole NodeRole

			if lf.config.DisableENICheck {
				// Use forced role for testing
				if lf.config.ForceRole == RoleStringPrimary {
					newRole = RolePrimary
					lf.logger.Info().Msg("ENI check disabled - forcing role to PRIMARY")
				} else {
					newRole = RoleSecondary
					lf.logger.Info().Msg("ENI check disabled - forcing role to SECONDARY")
				}
			} else {
				// Normal AWS ENI ownership check
				lf.logger.Debug().Str("eni_ip", lf.config.ENIIP).Msg("Checking ENI ownership")
				owns, err := lf.awsClient.CheckENIOwnership(ctx, lf.config.ENIIP)
				if err != nil {
					lf.logger.Error().Err(err).Str("eni_ip", lf.config.ENIIP).Msg("Failed to check ENI ownership")
					continue
				}

				lf.logger.Info().
					Str("eni_ip", lf.config.ENIIP).
					Bool("owns_eni", owns).
					Msg("ENI ownership check result")

				newRole = RoleSecondary
				if owns {
					newRole = RolePrimary
				}
			}

			if newRole != lf.currentRole {
				lf.logger.Info().
					Str("old_role", lf.currentRole.String()).
					Str("new_role", newRole.String()).
					Bool("disable_eni_check", lf.config.DisableENICheck).
					Msg("Role change detected, triggering transition")

				select {
				case lf.roleCh <- newRole:
					lf.logger.Debug().Str("new_role", newRole.String()).Msg("Role change sent to transition channel")
				default:
					lf.logger.Warn().Str("new_role", newRole.String()).Msg("Role transition channel full, skipping update")
				}
			} else {
				lf.logger.Debug().
					Str("current_role", lf.currentRole.String()).
					Str("determined_role", newRole.String()).
					Msg("Role unchanged, no transition needed")
			}
		}
	}
}

// roleManagementLoop handles transitions between primary and secondary roles
func (lf *LeaderFailover) roleManagementLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-lf.stopCh:
			return
		case newRole := <-lf.roleCh:
			lf.logger.Info().
				Str("current_role", lf.currentRole.String()).
				Str("target_role", newRole.String()).
				Msg("Received role transition request, starting transition")

			if err := lf.transitionToRole(ctx, newRole); err != nil {
				lf.logger.Error().Err(err).
					Str("current_role", lf.currentRole.String()).
					Str("target_role", newRole.String()).
					Msg("Failed to transition to new role")
			} else {
				lf.currentRole = newRole
				lf.logger.Info().
					Str("role", newRole.String()).
					Msg("Successfully transitioned to new role")
			}
		}
	}
}

// transitionToRole handles the transition logic between roles
func (lf *LeaderFailover) transitionToRole(ctx context.Context, newRole NodeRole) error {
	lf.logger.Info().
		Str("target_role", newRole.String()).
		Msg("Starting role transition, cleaning up current state")

	// Clean up current role
	if err := lf.cleanup(); err != nil {
		lf.logger.Warn().Err(err).Str("target_role", newRole.String()).Msg("Error during role cleanup")
	} else {
		lf.logger.Debug().Str("target_role", newRole.String()).Msg("Role cleanup completed successfully")
	}

	switch newRole {
	case RolePrimary:
		lf.logger.Info().Msg("Transitioning to PRIMARY role")
		return lf.becomePrimary(ctx)
	case RoleSecondary:
		lf.logger.Info().Msg("Transitioning to SECONDARY role")
		return lf.becomeSecondary(ctx)
	default:
		lf.logger.Error().Str("invalid_role", string(rune(newRole))).Msg("Invalid role requested for transition")
		return fmt.Errorf("invalid role: %v", newRole)
	}
}

// becomePrimary sets up this node as the primary (leader)
func (lf *LeaderFailover) becomePrimary(ctx context.Context) error {
	lf.logger.Info().Uint16("port", lf.config.Port).Msg("Becoming primary, starting fRPC server")

	// Execute failover actions if AWS client is available
	if lf.awsClient != nil {
		if err := lf.executeFailoverActions(ctx); err != nil {
			lf.logger.Error().Err(err).Msg("Failed to execute failover actions")
			// Don't fail the transition - log error and continue
		}
	}

	// Create fRPC server with this LeaderFailover as the service implementation
	var err error
	lf.frpcServer, err = NewServer(lf, nil, lf.logger)
	if err != nil {
		return fmt.Errorf("failed to create fRPC server: %w", err)
	}

	// Start the server in a goroutine
	serverAddr := fmt.Sprintf(":%d", lf.config.Port)
	go func() {
		if err := lf.frpcServer.Start(serverAddr); err != nil {
			lf.logger.Error().Err(err).Msg("fRPC server failed")
		}
	}()

	lf.logger.Info().Str("addr", serverAddr).Msg("fRPC server started")

	// Stop any existing heartbeat monitoring (from when we were secondary)
	if lf.heartbeatStopCh != nil {
		close(lf.heartbeatStopCh)
		lf.heartbeatStopCh = nil
	}

	// Primary sends heartbeats, doesn't monitor them
	lf.heartbeatMutex.Lock()
	lf.lastHeartbeat = time.Time{}
	lf.missedHeartbeats = 0
	lf.heartbeatMutex.Unlock()

	return nil
}

// becomeSecondary sets up this node as the secondary (follower)
func (lf *LeaderFailover) becomeSecondary(ctx context.Context) error {
	lf.logger.Info().
		Str("eni_ip", lf.config.ENIIP).
		Uint16("port", lf.config.Port).
		Msg("Becoming secondary, connecting to primary")

	// Create fRPC client to connect to primary
	primaryAddr := fmt.Sprintf("%s:%d", lf.config.ENIIP, lf.config.Port)
	c, err := NewClient(nil, lf.logger)
	if err != nil {
		return fmt.Errorf("failed to create fRPC client: %w", err)
	}

	// Test connection to primary
	if err := c.Connect(primaryAddr); err != nil {
		lf.logger.Warn().Err(err).
			Str("primary_addr", primaryAddr).
			Msg("Failed to connect to primary, will retry during sync")
		// Don't fail here - we'll retry connection during sync attempts
	} else {
		lf.logger.Info().Str("primary_addr", primaryAddr).Msg("Connected to primary")
	}

	// Validate the client is properly initialized before assigning
	// Test that the client can be safely closed to avoid nil pointer issues later
	if c != nil {
		// Only assign to lf.frpcClient after successful creation and validation
		lf.frpcClient = c
	} else {
		return errors.New("failed to create valid fRPC client")
	}

	// Initialize heartbeat tracking
	lf.heartbeatMutex.Lock()
	lf.lastHeartbeat = time.Now() // Start with current time to give primary time to start
	lf.missedHeartbeats = 0
	lf.heartbeatMutex.Unlock()

	// Create heartbeat stop channel
	lf.heartbeatStopCh = make(chan struct{})

	// Start heartbeat monitoring
	go lf.heartbeatMonitorLoop(ctx)

	// Start sync loop for NAT state synchronization
	go lf.secondarySyncLoop(ctx)

	return nil
}

// secondarySyncLoop handles periodic state synchronization when acting as secondary
func (lf *LeaderFailover) secondarySyncLoop(ctx context.Context) {
	ticker := time.NewTicker(lf.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-lf.stopCh:
			return
		case <-ticker.C:
			if lf.currentRole != RoleSecondary {
				return // Stop if we're no longer secondary
			}

			if err := lf.syncFromPrimary(ctx); err != nil {
				lf.logger.Error().Err(err).Msg("Failed to sync state from primary")
			} else {
				lf.logger.Debug().Msg("Successfully synced state from primary")
			}
		}
	}
}

// syncFromPrimary fetches state from primary and applies it locally
func (lf *LeaderFailover) syncFromPrimary(ctx context.Context) error {
	if lf.frpcClient == nil {
		return errors.New("fRPC client not initialized")
	}

	// Create sync request
	requestID := fmt.Sprintf("sync_%d", time.Now().UnixNano())
	request := &FailoverSyncStateRequest{
		RequestId: requestID,
	}

	// Send request to primary via fRPC
	response, err := lf.frpcClient.FailoverService.SyncState(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to send sync request: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("primary returned error: %s", response.ErrorMessage)
	}

	if response.State == nil {
		return errors.New("primary returned empty state")
	}

	// Convert fRPC state to client.NATState
	natState := &client.NATState{
		IPs:         response.State.Ips,
		TCPInbound:  make([]client.NATKeyValuePair, len(response.State.TcpInbound)),
		TCPOutbound: make([]client.NATKeyValuePair, len(response.State.TcpOutbound)),
		UDPInbound:  make([]client.NATKeyValuePair, len(response.State.UdpInbound)),
		UDPOutbound: make([]client.NATKeyValuePair, len(response.State.UdpOutbound)),
		NATPorts:    make([]client.NATBitmapPair, len(response.State.NatPorts)),
	}

	// Convert key-value pairs
	for i, kv := range response.State.TcpInbound {
		natState.TCPInbound[i] = client.NATKeyValuePair{
			Key: client.NATKey{
				DestinationIP:   kv.Key.DestinationIp,
				DestinationPort: portUint32ToUint16(kv.Key.DestinationPort),
				SourceIP:        kv.Key.SourceIp,
				SourcePort:      portUint32ToUint16(kv.Key.SourcePort),
			},
			Value: client.NATValue{
				LastSeen:      kv.Value.LastSeen,
				TranslateIP:   kv.Value.TranslateIp,
				TranslatePort: portUint32ToUint16(kv.Value.TranslatePort),
			},
		}
	}
	for i, kv := range response.State.TcpOutbound {
		natState.TCPOutbound[i] = client.NATKeyValuePair{
			Key: client.NATKey{
				DestinationIP:   kv.Key.DestinationIp,
				DestinationPort: portUint32ToUint16(kv.Key.DestinationPort),
				SourceIP:        kv.Key.SourceIp,
				SourcePort:      portUint32ToUint16(kv.Key.SourcePort),
			},
			Value: client.NATValue{
				LastSeen:      kv.Value.LastSeen,
				TranslateIP:   kv.Value.TranslateIp,
				TranslatePort: portUint32ToUint16(kv.Value.TranslatePort),
			},
		}
	}
	for i, kv := range response.State.UdpInbound {
		natState.UDPInbound[i] = client.NATKeyValuePair{
			Key: client.NATKey{
				DestinationIP:   kv.Key.DestinationIp,
				DestinationPort: portUint32ToUint16(kv.Key.DestinationPort),
				SourceIP:        kv.Key.SourceIp,
				SourcePort:      portUint32ToUint16(kv.Key.SourcePort),
			},
			Value: client.NATValue{
				LastSeen:      kv.Value.LastSeen,
				TranslateIP:   kv.Value.TranslateIp,
				TranslatePort: portUint32ToUint16(kv.Value.TranslatePort),
			},
		}
	}
	for i, kv := range response.State.UdpOutbound {
		natState.UDPOutbound[i] = client.NATKeyValuePair{
			Key: client.NATKey{
				DestinationIP:   kv.Key.DestinationIp,
				DestinationPort: portUint32ToUint16(kv.Key.DestinationPort),
				SourceIP:        kv.Key.SourceIp,
				SourcePort:      portUint32ToUint16(kv.Key.SourcePort),
			},
			Value: client.NATValue{
				LastSeen:      kv.Value.LastSeen,
				TranslateIP:   kv.Value.TranslateIp,
				TranslatePort: portUint32ToUint16(kv.Value.TranslatePort),
			},
		}
	}
	for i, bp := range response.State.NatPorts {
		var destIP *string
		if bp.DestinationIp != "" {
			destIP = &bp.DestinationIp
		}
		natState.NATPorts[i] = client.NATBitmapPair{
			Bitmap:        bp.Bitmap,
			DestinationIP: destIP,
			LastChunk:     portUint32ToUint16(bp.LastChunk),
			NATIP:         bp.Natip,
		}
	}

	// Apply the state to local conduit instance
	if err := lf.applySyncedState(ctx, natState); err != nil {
		return fmt.Errorf("failed to apply synced state: %w", err)
	}

	return nil
}

// cleanup releases resources from the current role
func (lf *LeaderFailover) cleanup() error {
	var lastErr error

	// Stop fRPC server if running
	if lf.frpcServer != nil {
		if err := lf.frpcServer.Shutdown(); err != nil {
			lf.logger.Error().Err(err).Msg("Error shutting down fRPC server")
			lastErr = err
		}
		lf.frpcServer = nil
	}

	// Close fRPC client if running
	if lf.frpcClient != nil {
		// Safely close the client with error recovery
		func() {
			defer func() {
				if r := recover(); r != nil {
					lf.logger.Error().Str("panic", fmt.Sprintf("%v", r)).Msg("Recovered from panic while closing fRPC client")
				}
			}()
			if err := lf.frpcClient.Close(); err != nil {
				lf.logger.Error().Err(err).Msg("Error closing fRPC client")
				lastErr = err
			}
		}()
		lf.frpcClient = nil
	}

	return lastErr
}

// SyncState implements the FailoverService interface for primary nodes
func (lf *LeaderFailover) SyncState(
	ctx context.Context,
	req *FailoverSyncStateRequest,
) (*FailoverSyncStateResponse, error) {
	lf.logger.Debug().Str("request_id", req.RequestId).Msg("Handling sync state request")

	// Get current NAT state from local conduit instance
	resp, err := lf.localClient.GetStateWithResponse(ctx)
	if err != nil {
		lf.logger.Error().Err(err).Msg("Failed to get local NAT state")
		return &FailoverSyncStateResponse{
			RequestId:    req.RequestId,
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to get state: %v", err),
		}, nil
	}

	if resp.StatusCode() != http.StatusOK {
		lf.logger.Error().Int("status", resp.StatusCode()).Msg("Local API returned error status")
		return &FailoverSyncStateResponse{
			RequestId:    req.RequestId,
			Success:      false,
			ErrorMessage: fmt.Sprintf("API error: %d", resp.StatusCode()),
		}, nil
	}

	if resp.JSON200 == nil {
		lf.logger.Error().Msg("Local API returned empty state")
		return &FailoverSyncStateResponse{
			RequestId:    req.RequestId,
			Success:      false,
			ErrorMessage: "empty state returned",
		}, nil
	}

	// Convert client.NATState to fRPC state
	natState := resp.JSON200
	frpcState := &FailoverNATState{
		Ips:         natState.IPs,
		TcpInbound:  make([]*FailoverNATKeyValuePair, len(natState.TCPInbound)),
		TcpOutbound: make([]*FailoverNATKeyValuePair, len(natState.TCPOutbound)),
		UdpInbound:  make([]*FailoverNATKeyValuePair, len(natState.UDPInbound)),
		UdpOutbound: make([]*FailoverNATKeyValuePair, len(natState.UDPOutbound)),
		NatPorts:    make([]*FailoverNATBitmapPair, len(natState.NATPorts)),
	}

	// Convert key-value pairs
	for i, kv := range natState.TCPInbound {
		frpcState.TcpInbound[i] = &FailoverNATKeyValuePair{
			Key: &FailoverNATKey{
				DestinationIp:   kv.Key.DestinationIP,
				DestinationPort: uint32(kv.Key.DestinationPort),
				SourceIp:        kv.Key.SourceIP,
				SourcePort:      uint32(kv.Key.SourcePort),
			},
			Value: &FailoverNATValue{
				LastSeen:      kv.Value.LastSeen,
				TranslateIp:   kv.Value.TranslateIP,
				TranslatePort: uint32(kv.Value.TranslatePort),
			},
		}
	}
	for i, kv := range natState.TCPOutbound {
		frpcState.TcpOutbound[i] = &FailoverNATKeyValuePair{
			Key: &FailoverNATKey{
				DestinationIp:   kv.Key.DestinationIP,
				DestinationPort: uint32(kv.Key.DestinationPort),
				SourceIp:        kv.Key.SourceIP,
				SourcePort:      uint32(kv.Key.SourcePort),
			},
			Value: &FailoverNATValue{
				LastSeen:      kv.Value.LastSeen,
				TranslateIp:   kv.Value.TranslateIP,
				TranslatePort: uint32(kv.Value.TranslatePort),
			},
		}
	}
	for i, kv := range natState.UDPInbound {
		frpcState.UdpInbound[i] = &FailoverNATKeyValuePair{
			Key: &FailoverNATKey{
				DestinationIp:   kv.Key.DestinationIP,
				DestinationPort: uint32(kv.Key.DestinationPort),
				SourceIp:        kv.Key.SourceIP,
				SourcePort:      uint32(kv.Key.SourcePort),
			},
			Value: &FailoverNATValue{
				LastSeen:      kv.Value.LastSeen,
				TranslateIp:   kv.Value.TranslateIP,
				TranslatePort: uint32(kv.Value.TranslatePort),
			},
		}
	}
	for i, kv := range natState.UDPOutbound {
		frpcState.UdpOutbound[i] = &FailoverNATKeyValuePair{
			Key: &FailoverNATKey{
				DestinationIp:   kv.Key.DestinationIP,
				DestinationPort: uint32(kv.Key.DestinationPort),
				SourceIp:        kv.Key.SourceIP,
				SourcePort:      uint32(kv.Key.SourcePort),
			},
			Value: &FailoverNATValue{
				LastSeen:      kv.Value.LastSeen,
				TranslateIp:   kv.Value.TranslateIP,
				TranslatePort: uint32(kv.Value.TranslatePort),
			},
		}
	}
	for i, bp := range natState.NATPorts {
		destIP := ""
		if bp.DestinationIP != nil {
			destIP = *bp.DestinationIP
		}
		frpcState.NatPorts[i] = &FailoverNATBitmapPair{
			Bitmap:        bp.Bitmap,
			DestinationIp: destIP,
			LastChunk:     uint32(bp.LastChunk),
			Natip:         bp.NATIP,
		}
	}

	return &FailoverSyncStateResponse{
		RequestId: req.RequestId,
		Success:   true,
		State:     frpcState,
	}, nil
}

// HealthCheck implements the FailoverService interface for health checking
func (lf *LeaderFailover) HealthCheck(
	_ context.Context,
	req *FailoverHealthCheckRequest,
) (*FailoverHealthCheckResponse, error) {
	return &FailoverHealthCheckResponse{
		RequestId:  req.RequestId,
		Success:    true,
		NodeRole:   lf.currentRole.String(),
		InstanceId: lf.awsClient.GetInstanceID(),
	}, nil
}

// applySyncedState applies the received state to the local conduit instance
func (lf *LeaderFailover) applySyncedState(ctx context.Context, state *client.NATState) error {
	resp, err := lf.localClient.SetStateWithResponse(ctx, *state)
	if err != nil {
		return fmt.Errorf("failed to set local state: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("local API returned error status: %d", resp.StatusCode())
	}

	lf.logger.Debug().
		Int("ips", len(state.IPs)).
		Int("tcp_inbound", len(state.TCPInbound)).
		Int("tcp_outbound", len(state.TCPOutbound)).
		Int("udp_inbound", len(state.UDPInbound)).
		Int("udp_outbound", len(state.UDPOutbound)).
		Int("nat_ports", len(state.NATPorts)).
		Msg("Applied synced state to local instance")

	return nil
}

// Heartbeat implements the FailoverService interface for heartbeat handling
func (lf *LeaderFailover) Heartbeat(
	_ context.Context,
	req *FailoverHeartbeatRequest,
) (*FailoverHeartbeatResponse, error) {
	// Only secondary should receive heartbeats from primary
	if lf.currentRole != RoleSecondary {
		return &FailoverHeartbeatResponse{
			RequestId: req.RequestId,
			Success:   false,
			Timestamp: time.Now().UnixNano(),
		}, errors.New("node is not secondary")
	}

	// Update heartbeat tracking
	lf.heartbeatMutex.Lock()
	lf.lastHeartbeat = time.Now()
	lf.missedHeartbeats = 0
	lf.heartbeatMutex.Unlock()

	return &FailoverHeartbeatResponse{
		RequestId: req.RequestId,
		Success:   true,
		Timestamp: time.Now().UnixNano(),
	}, nil
}

// executeFailoverActions performs route table updates and floating IP reassignment
func (lf *LeaderFailover) executeFailoverActions(ctx context.Context) error {
	lf.logger.Info().Msg("Executing failover actions")

	// First, take over the ENI IP if we don't already own it
	if err := lf.awsClient.TakeOverENI(ctx, lf.config.ENIIP); err != nil {
		lf.logger.Error().Err(err).Msg("Failed to take over ENI IP")
		return fmt.Errorf("failed to take over ENI IP: %w", err)
	}

	lf.logger.Info().Str("eni_ip", lf.config.ENIIP).Msg("Successfully took over ENI IP")

	// Get the ENI ID for this instance (new primary)
	newENI, err := lf.awsClient.GetENIByIP(ctx, lf.config.ENIIP)
	if err != nil {
		return fmt.Errorf("failed to get new primary ENI: %w", err)
	}

	lf.logger.Info().Str("new_eni", newENI).Msg("Identified new primary ENI")

	// Find the old primary ENI (the one that currently has floating IPs)
	// We'll look for ENIs with floating IPs in the same subnet
	oldENI := ""
	var floatingIPs []string

	// Get all ENIs in the same VPC to find the one with floating IPs
	describeInput := &ec2.DescribeNetworkInterfacesInput{}
	result, err := lf.awsClient.EC2Client.DescribeNetworkInterfaces(ctx, describeInput)
	if err != nil {
		lf.logger.Warn().Err(err).Msg("Failed to describe network interfaces")
	} else {
		for _, eni := range result.NetworkInterfaces {
			if eni.NetworkInterfaceId != nil && *eni.NetworkInterfaceId != newENI {
				// Get floating IPs for this ENI
				ips, err := lf.awsClient.GetENIFloatingIPs(ctx, *eni.NetworkInterfaceId)
				if err != nil {
					continue
				}

				// Filter to only get IPs matching the pattern (x.x.x.20 onwards)
				filtered := lf.awsClient.FilterFloatingIPs(ips, lf.config.ENIIP)
				if len(filtered) > 0 {
					oldENI = *eni.NetworkInterfaceId
					floatingIPs = filtered
					lf.logger.Info().
						Str("old_eni", oldENI).
						Int("floating_ip_count", len(floatingIPs)).
						Str("floating_ips", strings.Join(floatingIPs, ",")).
						Msg("Found old primary ENI with floating IPs")
					break
				}
			}
		}
	}

	// Create error channel for parallel operations
	errCh := make(chan error, 2)

	// Execute route table update and floating IP reassignment in parallel
	go func() {
		if lf.config.DestinationCIDR != "" {
			lf.logger.Info().
				Str("cidr", lf.config.DestinationCIDR).
				Str("new_eni", newENI).
				Msg("Updating route tables")

			if err := lf.awsClient.UpdateRouteTables(ctx, lf.config.DestinationCIDR, newENI); err != nil {
				errCh <- fmt.Errorf("route table update failed: %w", err)
			} else {
				lf.logger.Info().Msg("Route tables updated successfully")
				errCh <- nil
			}
		} else {
			lf.logger.Warn().Msg("No destination CIDR configured, skipping route table update")
			errCh <- nil
		}
	}()

	// Reassign floating IPs
	go func() {
		if oldENI != "" && len(floatingIPs) > 0 {
			lf.logger.Info().
				Str("old_eni", oldENI).
				Str("new_eni", newENI).
				Int("ip_count", len(floatingIPs)).
				Str("ips", strings.Join(floatingIPs, ",")).
				Msg("Reassigning floating IPs")

			if err := lf.awsClient.ReassignFloatingIPs(ctx, oldENI, newENI, floatingIPs); err != nil {
				errCh <- fmt.Errorf("floating IP reassignment failed: %w", err)
			} else {
				lf.logger.Info().Msg("Floating IPs reassigned successfully")
				errCh <- nil
			}
		} else {
			lf.logger.Info().Msg("No floating IPs to reassign")
			errCh <- nil
		}
	}()

	// Wait for both operations to complete
	var errs []string
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failover action errors: %s", strings.Join(errs, "; "))
	}

	lf.currentENI = newENI
	lf.logger.Info().Msg("Failover actions completed successfully")
	return nil
}

// heartbeatMonitorLoop monitors for missed heartbeats when acting as secondary
func (lf *LeaderFailover) heartbeatMonitorLoop(ctx context.Context) {
	ticker := time.NewTicker(lf.config.HeartbeatInterval)
	defer ticker.Stop()

	lf.logger.Info().
		Str("interval", lf.config.HeartbeatInterval.String()).
		Int("threshold", lf.config.HeartbeatMissThreshold).
		Msg("Starting heartbeat monitor")

	for {
		select {
		case <-ctx.Done():
			return
		case <-lf.stopCh:
			return
		case <-lf.heartbeatStopCh:
			return
		case <-ticker.C:
			lf.heartbeatMutex.Lock()
			timeSinceLastHeartbeat := time.Since(lf.lastHeartbeat)

			// Check if we've missed a heartbeat
			if timeSinceLastHeartbeat > lf.config.HeartbeatInterval {
				lf.missedHeartbeats++
				lf.logger.Warn().
					Int("missed_count", lf.missedHeartbeats).
					Str("time_since_last", timeSinceLastHeartbeat.String()).
					Msg("Missed heartbeat from primary")

				// Check if we've hit the threshold
				if lf.missedHeartbeats >= lf.config.HeartbeatMissThreshold {
					lf.logger.Error().
						Int("missed_count", lf.missedHeartbeats).
						Msg("Heartbeat threshold exceeded, initiating failover")

					// Reset before unlock to avoid race
					lf.missedHeartbeats = 0
					lf.heartbeatMutex.Unlock()

					// Trigger failover to primary
					select {
					case lf.roleCh <- RolePrimary:
						lf.logger.Info().Msg("Triggered failover to primary role")
					default:
						lf.logger.Warn().Msg("Role channel full, failover request dropped")
					}
					return
				}
			}
			lf.heartbeatMutex.Unlock()
		}
	}
}
