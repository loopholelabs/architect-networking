# Transit Configuration Guide

## Overview

This guide explains how to configure Conduit's Transit dataplane, particularly for customers using the pre-compiled dataplane binary. Transit can be configured through:

1. **Configuration File** (YAML/JSON) - Initial setup
2. **Environment Variables** - Override specific settings
3. **HTTP API** - Runtime modifications

## Pre-compiled Dataplane Usage

When using the pre-compiled Conduit binary, you don't need to import the Go packages. Instead, you:

1. Create a configuration file
2. Run the binary with your configuration
3. Use the HTTP API for runtime changes

### Basic Usage

```bash
# Run with configuration file
conduit transit --config /path/to/config.yaml

# Run with environment variable overrides
CONDUIT_TRANSIT_CONFIG_INTERFACE_MAC="aa:bb:cc:dd:ee:ff" \
CONDUIT_TRANSIT_CONFIG_DEFAULT_DESTINATION_MAC="11:22:33:44:55:66" \
conduit transit --config /path/to/config.yaml
```

## Configuration File Structure

The complete configuration file has three main sections:

```yaml
# Transit dataplane configuration
transit_config:
  # ... transit settings

# HTTP API server configuration  
server_config:
  # ... server settings

# Optional StatsD metrics configuration
statsd_config:
  # ... metrics settings
```

## Transit Configuration Reference

### Complete Configuration Example

```yaml
transit_config:
  # Required: Network interface configuration
  interface_mac: "aa:bb:cc:dd:ee:ff"
  default_destination_mac: "11:22:33:44:55:66"
  
  # Optional: Management IP that bypasses NAT
  management_ip: "50.0.0.1"
  
  # DDoS Protection Configuration
  disable_ddos: false  # Enable DDoS protection
  initial_ddos_ips:    # IPs to block at startup
    - "192.168.1.100"
    - "10.0.0.50"
  
  # Firewall Configuration
  disable_icmp_firewall: false
  disable_tcp_firewall: false
  disable_udp_firewall: false
  
  # Initial firewall rules
  initial_icmp_rules:
    - source_cidr: "10.0.0.0/24"
      destination_cidr: "192.168.0.0/16"
    - source_cidr: "172.16.0.0/12"
      destination_cidr: "0.0.0.0/0"
  
  initial_tcp_rules:
    - source_cidr: "0.0.0.0/0"
      destination_cidr: "10.0.0.0/8"
      source_port_low: 1024
      source_port_high: 65535
      destination_port_low: 443
      destination_port_high: 443
    - source_cidr: "10.0.0.0/8"
      destination_cidr: "0.0.0.0/0"
      source_port_low: 0
      source_port_high: 65535
      destination_port_low: 80
      destination_port_high: 80
  
  initial_udp_rules:
    - source_cidr: "0.0.0.0/0"
      destination_cidr: "0.0.0.0/0"
      source_port_low: 0
      source_port_high: 65535
      destination_port_low: 53
      destination_port_high: 53
  
  # Router/NAT Configuration
  disable_router: false
  disable_outbound_nat: false
  disable_inbound_nat: true  # Inbound NAT not implemented
  disable_interfaces: true   # Multi-interface support
  
  initial_nat_ips:
    - "100.64.0.1"
    - "100.64.0.2"
    - "100.64.0.3"
  
  # Timeout configuration (in milliseconds)
  tcp_timeout: 86400000  # 24 hours
  udp_timeout: 180000    # 3 minutes
  gc_interval: 300000    # 5 minutes
  
  # ARP Configuration
  disable_arp: false
  
  # Event System Configuration
  disable_arp_events: false
  disable_icmp_events: false
  disable_udp_events: false
  disable_tcp_events: false
  
  # Interface Configuration (for future use)
  interfaces: []
  generic_mode: false

# HTTP API Server Configuration
server_config:
  httpAddr: "/unix/tmp/conduit.sock"  # Unix socket (recommended)
  # httpAddr: "/ip4/127.0.0.1/tcp/8080"  # TCP alternative

# Optional StatsD Configuration
statsd_config:
  addr: "127.0.0.1:8125"
  prefix: "conduit"
  tags:
    environment: "production"
    region: "us-west-2"
```

### Configuration Parameters

#### Required Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `interface_mac` | string | MAC address of the network interface (format: `aa:bb:cc:dd:ee:ff`) |
| `default_destination_mac` | string | Default destination MAC for packets |

#### Optional Parameters

##### Basic Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `management_ip` | string | "" | IP that bypasses NAT (e.g., for management traffic) |

##### DDoS Protection

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `disable_ddos` | bool | true | Disable DDoS protection |
| `initial_ddos_ips` | []string | [] | IP addresses to block at startup |

##### Firewall Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `disable_icmp_firewall` | bool | true | Disable ICMP firewall |
| `disable_tcp_firewall` | bool | true | Disable TCP firewall |
| `disable_udp_firewall` | bool | true | Disable UDP firewall |
| `initial_icmp_rules` | []ICMPRule | [] | ICMP firewall rules |
| `initial_tcp_rules` | []FirewallRule | [] | TCP firewall rules |
| `initial_udp_rules` | []FirewallRule | [] | UDP firewall rules |

##### Router/NAT Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `disable_router` | bool | true | Disable router functionality |
| `disable_outbound_nat` | bool | true | Disable outbound NAT |
| `disable_inbound_nat` | bool | true | Disable inbound NAT (not implemented) |
| `disable_interfaces` | bool | true | Disable multi-interface support |
| `initial_nat_ips` | []string | [] | NAT IP addresses |
| `tcp_timeout` | uint64 | 86400000 | TCP connection timeout (ms) |
| `udp_timeout` | uint64 | 180000 | UDP connection timeout (ms) |
| `gc_interval` | uint64 | 300000 | Garbage collection interval (ms) |

##### Event Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `disable_arp_events` | bool | true | Disable ARP event tracking |
| `disable_icmp_events` | bool | true | Disable ICMP event tracking |
| `disable_udp_events` | bool | true | Disable UDP event tracking |
| `disable_tcp_events` | bool | true | Disable TCP event tracking |

## Firewall Rule Format

### ICMP Rules

ICMP rules only support IP-based filtering:

```yaml
initial_icmp_rules:
  - source_cidr: "10.0.0.0/24"      # Source IP range
    destination_cidr: "0.0.0.0/0"   # Destination IP range
```

### TCP/UDP Rules

TCP and UDP rules support both IP and port filtering:

```yaml
initial_tcp_rules:
  - source_cidr: "0.0.0.0/0"        # Any source IP
    destination_cidr: "10.0.0.0/8"  # Destination subnet
    source_port_low: 1024           # Source port range start
    source_port_high: 65535         # Source port range end
    destination_port_low: 443       # Destination port (HTTPS)
    destination_port_high: 443      # Same value for single port
```

#### Special Cases

```yaml
# Allow all ports (0-65535)
- source_cidr: "10.0.0.0/8"
  destination_cidr: "0.0.0.0/0"
  source_port_low: 0
  source_port_high: 65535
  destination_port_low: 0
  destination_port_high: 65535

# Single source port, range of destination ports
- source_cidr: "192.168.1.0/24"
  destination_cidr: "10.0.0.0/8"
  source_port_low: 53
  source_port_high: 53
  destination_port_low: 1024
  destination_port_high: 65535
```

## Environment Variable Override

Any configuration parameter can be overridden using environment variables:

```bash
# Format: CONDUIT_TRANSIT_CONFIG_<PARAMETER_PATH>
# Nested parameters use underscores

# Basic settings
export CONDUIT_TRANSIT_CONFIG_INTERFACE_MAC="aa:bb:cc:dd:ee:ff"
export CONDUIT_TRANSIT_CONFIG_MANAGEMENT_IP="50.0.0.1"

# Enable features
export CONDUIT_TRANSIT_CONFIG_DISABLE_ROUTER=false
export CONDUIT_TRANSIT_CONFIG_DISABLE_TCP_FIREWALL=false

# Timeouts
export CONDUIT_TRANSIT_CONFIG_TCP_TIMEOUT=43200000  # 12 hours in ms

# Server configuration
export CONDUIT_SERVER_CONFIG_HTTPADDR="/ip4/0.0.0.0/tcp/8080"
```

## Common Deployment Scenarios

### 1. NAT Gateway

```yaml
transit_config:
  interface_mac: "02:00:00:00:00:01"
  default_destination_mac: "02:00:00:00:00:02"
  
  # Enable router and outbound NAT
  disable_router: false
  disable_outbound_nat: false
  
  # Configure NAT IPs
  initial_nat_ips:
    - "100.64.0.1"
    - "100.64.0.2"
    - "100.64.0.3"
  
  # Adjust timeouts for your workload
  tcp_timeout: 86400000  # 24 hours
  udp_timeout: 180000    # 3 minutes
```

### 2. DDoS Protection Layer

```yaml
transit_config:
  interface_mac: "02:00:00:00:00:01"
  default_destination_mac: "02:00:00:00:00:02"
  
  # Enable DDoS protection
  disable_ddos: false
  
  # Pre-populate with known bad actors
  initial_ddos_ips:
    - "192.168.1.100"
    - "10.0.0.50"
  
  # Enable events for monitoring
  disable_tcp_events: false
  disable_udp_events: false
```

### 3. Stateful Firewall

```yaml
transit_config:
  interface_mac: "02:00:00:00:00:01"
  default_destination_mac: "02:00:00:00:00:02"
  
  # Enable all firewalls
  disable_icmp_firewall: false
  disable_tcp_firewall: false
  disable_udp_firewall: false
  
  # Allow established connections back
  initial_tcp_rules:
    # Allow inbound HTTPS to servers
    - source_cidr: "0.0.0.0/0"
      destination_cidr: "10.0.0.0/24"
      source_port_low: 1024
      source_port_high: 65535
      destination_port_low: 443
      destination_port_high: 443
    
    # Allow outbound HTTP/HTTPS
    - source_cidr: "10.0.0.0/24"
      destination_cidr: "0.0.0.0/0"
      source_port_low: 1024
      source_port_high: 65535
      destination_port_low: 80
      destination_port_high: 443
```

### 4. Minimal Configuration

For basic packet forwarding without any features:

```yaml
transit_config:
  interface_mac: "02:00:00:00:00:01"
  default_destination_mac: "02:00:00:00:00:02"
  # All features disabled by default

server_config:
  httpAddr: "/unix/tmp/conduit.sock"
```

## Validation and Troubleshooting

### Configuration Validation

The transit binary validates configuration at startup:

```bash
# Test configuration without starting
conduit transit --config config.yaml --validate-only

# Run with verbose logging
conduit transit --config config.yaml --log-level debug
```

### Common Issues

1. **Invalid MAC Address Format**
   ```
   Error: invalid MAC address: "aa-bb-cc-dd-ee-ff"
   Fix: Use colon separator: "aa:bb:cc:dd:ee:ff"
   ```

2. **Port Range Validation**
   ```
   Error: source_port_high (1023) must be >= source_port_low (1024)
   Fix: Ensure high port >= low port
   ```

3. **CIDR Format**
   ```
   Error: invalid CIDR: "10.0.0.0"
   Fix: Include network mask: "10.0.0.0/24"
   ```

4. **Too Many Firewall Rules**
   ```
   Error: too many firewall rules (max 256)
   Fix: Consolidate rules or use broader CIDR ranges
   ```

## Runtime Configuration via HTTP API

After starting with initial configuration, use the HTTP API for runtime changes:

```bash
# Add NAT IP at runtime
curl -X POST "http://localhost:8080/v1/transit/ips?ip=100.64.0.4"

# Enable TCP firewall
curl -X PUT http://localhost:8080/v1/transit/firewall/tcp \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}'

# Add firewall rule
curl -X POST http://localhost:8080/v1/transit/firewall/tcp/rules \
  -H "Content-Type: application/json" \
  -d '{
    "source_cidr": "10.0.0.0/8",
    "destination_cidr": "0.0.0.0/0",
    "destination_port_low": 22,
    "destination_port_high": 22
  }'
```

See the [HTTP API Documentation](http-api.md) for complete API reference.

## Best Practices

1. **Start with minimal configuration** - Enable only needed features
2. **Use configuration files** - Easier to manage than environment variables
3. **Plan firewall rules carefully** - Maximum 256 rules per protocol
4. **Monitor resource usage** - Use events and counters for visibility
5. **Test with flow command** - Validate configuration before deployment
6. **Use management IP** - Ensure management traffic bypasses NAT
7. **Set appropriate timeouts** - Balance resource usage and connection stability