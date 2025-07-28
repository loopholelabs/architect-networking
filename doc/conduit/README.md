# Conduit Documentation

This directory contains detailed documentation for Conduit's core components and functionality.

## Documentation Overview

### [Transit Implementation Guide](transit-implementation.md)
Comprehensive guide to Conduit's eBPF dataplane implementation, covering:
- **Firewall System**: Protocol-specific rules, caching, and 4D matching
- **DDoS Mitigation**: Bloom filter-based high-performance IP blocking
- **Event System**: Real-time packet processing visibility
- **Router with NAT**: Connection tracking, port management, and timeout handling

### [Transit Configuration Guide](transit-configuration.md)
Detailed configuration reference for the pre-compiled dataplane:
- **Configuration File Structure**: YAML/JSON format and parameters
- **Environment Variables**: Override configuration at runtime
- **Common Deployment Scenarios**: NAT gateway, DDoS protection, firewall
- **Validation and Troubleshooting**: Common issues and solutions

### [HTTP API Documentation](http-api.md)
Complete reference for the RESTful API that provides runtime control over Conduit:
- IP address management
- DDoS protection control
- Firewall rule configuration
- NAT state import/export
- OpenAPI specification details

### [Flow Command Guide](flow-command.md)
Tutorial and reference for the packet testing tool:
- Processing pcap files through the dataplane
- Debugging packet drops and NAT behavior
- Testing firewall rules and DDoS protection
- Integration with development workflows

## Quick Start

1. **Initial Setup**: Create a configuration file for the pre-compiled binary (see [Transit Configuration](transit-configuration.md))
2. **Runtime Management**: Use the HTTP API to modify settings without restart (see [HTTP API](http-api.md))
3. **Testing & Debugging**: Use the flow command to validate configurations (see [Flow Command](flow-command.md))

### Minimal Example

```yaml
# config.yaml
transit_config:
  interface_mac: "aa:bb:cc:dd:ee:ff"
  default_destination_mac: "11:22:33:44:55:66"
  disable_router: false
  disable_outbound_nat: false
  initial_nat_ips: ["100.64.0.1"]

server_config:
  httpAddr: "/unix/tmp/conduit.sock"
```

```bash
# Run the pre-compiled binary
conduit transit --config config.yaml

# Add NAT IP via API
curl -X POST "/unix/tmp/conduit.sock/v1/transit/ips?ip=100.64.0.2"
```

## Architecture Summary

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Transit       │     │   HTTP API      │     │   Flow Tool     │
│  (eBPF Core)    │◄────│   (Runtime)     │     │   (Testing)     │
└─────────────────┘     └─────────────────┘     └─────────────────┘
        │                                                 │
        │                                                 │
        ▼                                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                        XDP eBPF Program                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐   │
│  │ Firewall │  │   DDoS   │  │  Events  │  │ Router (NAT) │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Key Concepts

### Initial vs Runtime Configuration
- **Initial**: Set via Transit config at startup
- **Runtime**: Modified via HTTP API without restart
- Both affect the same underlying eBPF dataplane

### Performance First Design
- eBPF processing at XDP layer (earliest possible)
- Bitmap-based firewall rules for O(1) lookups
- Bloom filters for DDoS protection
- Caching to reduce repeated computations

### Scalability Limits
- Firewall: 256 rules per protocol
- DDoS: 10,000 blocked IPs
- NAT: 134 million total connections, ~64k per IP/destination pair
- Events: Can be disabled per-protocol for performance

## Common Use Cases

### 1. DDoS Protection
```yaml
# Initial config
disable_ddos: false
initial_ddos_ips: ["192.168.1.100"]

# Runtime via API
POST /transit/ddos/ips?ip=10.0.0.50
```

### 2. NAT Gateway
```yaml
# Initial config
disable_router: false
disable_outbound_nat: false
initial_nat_ips: ["100.64.0.1", "100.64.0.2"]

# Runtime via API
POST /transit/ips?ip=100.64.0.3
```

### 3. Stateful Firewall
```yaml
# Initial config
initial_tcp_rules:
  - source_cidr: "10.0.0.0/8"
    destination_cidr: "0.0.0.0/0"
    destination_port_low: 443
    destination_port_high: 443

# Runtime via API
POST /transit/firewall/tcp/rules
```

## Additional Resources

- [OpenAPI Specification](../api/rest/v1/openapi.yaml): Machine-readable API definition
- [Example Configurations](../examples/): Sample config files for common scenarios
- [Integration Tests](../pkg/server/http/integration_test/): Examples of API usage