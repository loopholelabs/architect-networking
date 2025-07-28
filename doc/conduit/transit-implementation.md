# Transit Implementation Guide

## Overview

Transit is the core dataplane component of Conduit, implementing high-performance packet processing using eBPF. It provides a unified interface for network security features including firewalling, DDoS mitigation, NAT routing, and event monitoring. All components can be configured at startup through the `pkg/transit` package and dynamically modified at runtime via the HTTP API.

## Core Components

### 1. Firewall System

The firewall implementation provides granular packet filtering with protocol-specific rule management and performance optimizations.

#### Architecture

- **Rule Storage**: Uses bitmap-based indexing (0-255 rule positions) for efficient matching
- **Protocol Support**: Separate implementations for ICMP, TCP, and UDP traffic
- **Performance**: Caching mechanism reduces processing overhead by storing recent decisions

#### ICMP Firewall
- **Matching Dimensions**: Source IP and Destination IP only
- **Rule Format**: Supports CIDR notation for IP ranges (e.g., `10.0.0.0/24`)
- **Implementation**: 2D matching across source and destination IP addresses

#### TCP/UDP Firewall
- **Matching Dimensions**: 4D matching system
  - Source IP (with CIDR ranges)
  - Destination IP (with CIDR ranges)
  - Source Port (with port ranges)
  - Destination Port (with port ranges)
- **Rule Format**: Flexible rules supporting any combination of the four dimensions

#### Caching Mechanism
- **Purpose**: Reduce repeated rule evaluations for similar traffic patterns
- **Operation**: Caches firewall decisions until rules change
- **Benefit**: Significantly faster lookups for established connections

#### Configuration Example
```go
// Initial firewall rules via transit.Config
config := transit.DefaultConfig(interfaceMAC, destMAC)
config.DisableICMPFirewall = false
config.InitialICMPRules = []transit.ICMPFirewallRule{
    {
        SourceCIDR:      "10.0.0.0/24",
        DestinationCIDR: "192.168.0.0/16",
    },
}
config.InitialTCPRules = []transit.FirewallRule{
    {
        SourceCIDR:      "0.0.0.0/0",
        DestinationCIDR: "10.0.0.0/8",
        SourcePortLow:   1024,
        SourcePortHigh:  65535,
        DestPortLow:     443,
        DestPortHigh:    443,
    },
}
```

### 2. DDoS Mitigation

The DDoS protection system uses a hybrid approach combining bloom filters with exact matching for high-performance packet dropping.

#### Architecture
- **Bloom Filter**: Probabilistic data structure for fast IP checking
- **Exact Match Table**: Confirms bloom filter positives to eliminate false positives
- **Design Goal**: Handle millions of packets per second without affecting legitimate traffic

#### Key Characteristics
- **IP-Based**: Each individual IP address must be added manually (no range support)
- **High Performance**: Minimal CPU overhead even under attack
- **Reset-Only Removal**: Cannot remove individual IPs; must reset entire filter
- **Capacity**: Supports up to 10,000 blocked IP addresses

#### Usage Considerations
- Best suited for active attack mitigation
- Add attacker IPs as they're identified
- Reset the entire filter when attack subsides
- Not designed for general IP blacklisting (use firewall instead)

#### Configuration Example
```go
config.DisableDDoS = false
config.InitialDDoSIPs = []string{
    "192.168.1.100",
    "10.0.0.50",
}
```

### 3. Event System

The event system provides real-time visibility into packet processing decisions and system behavior.

#### Event Types
- **ARP Events**: ARP request/response processing
- **ICMP Events**: ICMP packet handling and decisions
- **TCP Events**: TCP connection establishment and state changes
- **UDP Events**: UDP packet processing

#### Dynamic Control
- Events can be enabled/disabled at runtime without restart
- Useful for debugging specific issues
- Minimal performance impact when disabled

#### Counter System
- Tracks various drop reasons and processing statistics
- Examples: `dropped_ddos`, `dropped_tcp_firewall`, `passed_reserved_port_tcp`
- Counters are continuously updated and can be exported

#### Configuration Example
```go
config.DisableARPEvents = true   // Start with ARP events disabled
config.DisableICMPEvents = false // Enable ICMP event tracking
config.DisableUDPEvents = false  // Enable UDP event tracking
config.DisableTCPEvents = false  // Enable TCP event tracking
```

### 4. Router with NAT Implementation

The router provides high-performance Network Address Translation (NAT) with sophisticated connection tracking and port management.

#### Outbound NAT Architecture

##### Port Allocation
- **Bitmap-Based Management**: Uses 64-bit chunks for efficient port tracking
- **Port Reuse**: Same port can be used across multiple NAT IPs to different destinations
- **Port Range**: Ports 1024-65535 available for allocation (configurable)
- **De Bruijn Optimization**: Fast bit scanning for available port selection

##### Connection Tracking
- **Bidirectional State**: Maintains both outbound and inbound mappings
- **State Machine**: Tracks TCP connection states (ESTABLISHED, FIN_WAIT, CLOSED)
- **Maximum Connections**: 
  - Total: Up to 134 million concurrent connections (2^27)
  - Per NAT IP to same destination: ~64,511 connections (ports 1024-65535)

##### Timeout Handling
- **TCP Timeouts**:
  - RST packets: Immediate connection closure
  - FIN packets: 2-minute timeout (FIN_WAIT state)
  - Normal connections: Configurable (default 24 hours)
- **UDP Timeouts**: Configurable (default 3 minutes)
- **Garbage Collection**: 
  - Runs periodically (default 5 minutes)
  - Checks BOTH inbound AND outbound `last_seen` timestamps
  - Only removes entries when both sides are stale

##### Special Features
- **Management IP Bypass**: Traffic to management IP bypasses NAT entirely
- **Reserved Ports**: Ports below 1024 pass through without NAT
- **Round-Robin IP Selection**: Distributes connections across available NAT IPs

#### Configuration Example
```go
config.DisableRouter = false
config.DisableOutboundNAT = false
config.InitialNATIPs = []string{
    "100.64.0.1",
    "100.64.0.2",
    "100.64.0.3",
}
config.TCPTimeout = 86400000 // 24 hours in milliseconds
config.UDPTimeout = 180000   // 3 minutes in milliseconds
config.GCInterval = 300000   // 5 minutes in milliseconds
config.ManagementIP = "50.0.0.1" // Management IP for bypass
```

#### NAT Limitations and Behavior
- **Port Exhaustion**: When all ports are allocated for a NAT IP to a specific destination, new connections will be dropped
- **IP Removal**: Removing a NAT IP invalidates all connections using it
- **State Persistence**: NAT state can be exported/imported for high availability
- **No Inbound NAT**: Currently only outbound NAT is implemented

## Initial Configuration

All components are configured through the `pkg/transit` package at startup:

```go
import (
    "github.com/loopholelabs/conduit/pkg/transit"
    "github.com/loopholelabs/logging"
)

// Create configuration
config := transit.DefaultConfig("aa:bb:cc:dd:ee:ff", "11:22:33:44:55:66")

// Configure all components
config.ManagementIP = "50.0.0.1"
config.DisableDDoS = false
config.DisableRouter = false
config.DisableOutboundNAT = false
// ... configure other components

// Create Transit instance
logger := logging.New(logging.Zerolog, "transit")
tr, err := transit.New(config, logger, eventEmitter)
if err != nil {
    log.Fatal(err)
}
defer tr.Close()

// Attach to network interface
prog := tr.GetProgram() // Returns eBPF XDP program
```

## Runtime Modification

While initial configuration is done at startup, all settings can be modified at runtime through the HTTP API. This allows for:
- Adding/removing firewall rules without restart
- Blocking IPs during active DDoS attacks
- Enabling/disabling event types for debugging
- Adding/removing NAT IPs
- Adjusting timeouts and other parameters

See the [HTTP API documentation](http-api.md) for details on runtime configuration.