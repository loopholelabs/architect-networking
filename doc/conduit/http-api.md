# HTTP API Documentation

## Overview

The Conduit HTTP API provides programmatic access to manage and configure the dataplane at runtime. It follows RESTful principles and uses JSON for request/response bodies. The API is defined using OpenAPI 3.0 specification, ensuring consistency and enabling client generation.

## Configuration

The HTTP API server is configured through the `server_config` section of the main configuration file:

```yaml
server_config:
  httpAddr: "/unix/tmp/conduit.sock"  # Unix socket (default)
  # httpAddr: "/ip4/127.0.0.1/tcp/8080"  # TCP socket alternative
```

### Configuration Options
- **Unix Socket** (recommended): `/unix/path/to/socket.sock`
- **TCP Socket**: `/ip4/127.0.0.1/tcp/8080`
- Uses multiaddr format for flexible addressing

## API Endpoints

### IP Management

#### List IPs
- **GET** `/transit/ips`
- Returns all IP addresses allocated to the dataplane
- Response: Array of IPv4 addresses

#### Add IP
- **POST** `/transit/ips?ip=192.168.1.100`
- Allocates a new IP address for NAT operations
- Returns updated list of all IPs
- Errors: 409 (already exists), 422 (invalid IP)

#### Remove IP
- **DELETE** `/transit/ips?ip=192.168.1.100`
- Removes an IP from the dataplane
- **Warning**: Affects active NAT connections using this IP
- Returns updated list of remaining IPs
- Errors: 404 (not found), 422 (invalid IP)

### DDoS Protection

#### Get Status
- **GET** `/transit/ddos`
- Returns DDoS protection status and statistics
- Response includes: enabled status, blocked IP count, capacity

#### Enable/Disable
- **PUT** `/transit/ddos`
- Body: `{"enabled": true}`
- Toggles DDoS protection on/off
- Blocked IP list is preserved when toggling

#### Reset Protection
- **DELETE** `/transit/ddos`
- Clears ALL blocked IP addresses
- Protection enabled/disabled status remains unchanged

#### Add Blocked IP
- **POST** `/transit/ddos/ips?ip=10.0.0.50`
- Adds an IP to the DDoS blocklist
- Individual IPs only (no ranges)
- Maximum 10,000 IPs

### Firewall Management

#### Global Firewall Status
- **GET** `/transit/firewall`
- Returns enabled/disabled status for all firewall types
- Shows ICMP, TCP, and UDP firewall states

#### ICMP Firewall

##### Status
- **GET** `/transit/firewall/icmp`
- Returns ICMP firewall status and rule count

##### Enable/Disable
- **PUT** `/transit/firewall/icmp`
- Body: `{"enabled": true}`
- Toggles ICMP firewall on/off

##### List Rules
- **GET** `/transit/firewall/icmp/rules`
- Returns all ICMP firewall rules with IDs

##### Add Rule
- **POST** `/transit/firewall/icmp/rules`
- Body:
  ```json
  {
    "source_cidr": "10.0.0.0/24",
    "destination_cidr": "192.168.0.0/16"
  }
  ```
- ICMP rules support only IP-based filtering

##### Delete Rule
- **DELETE** `/transit/firewall/icmp/rules/{ruleId}`
- Removes specific rule by ID

#### TCP Firewall

##### Status
- **GET** `/transit/firewall/tcp`
- Returns TCP firewall status and rule count

##### Enable/Disable
- **PUT** `/transit/firewall/tcp`
- Body: `{"enabled": true}`

##### List Rules
- **GET** `/transit/firewall/tcp/rules`
- Returns all TCP firewall rules with IDs

##### Add Rule
- **POST** `/transit/firewall/tcp/rules`
- Body:
  ```json
  {
    "source_cidr": "0.0.0.0/0",
    "destination_cidr": "10.0.0.0/8",
    "source_port_low": 1024,
    "source_port_high": 65535,
    "destination_port_low": 443,
    "destination_port_high": 443
  }
  ```
- Supports 4D matching: source/dest IPs and ports

##### Delete Rule
- **DELETE** `/transit/firewall/tcp/rules/{ruleId}`

#### UDP Firewall

UDP firewall endpoints follow the same pattern as TCP:
- **GET** `/transit/firewall/udp`
- **PUT** `/transit/firewall/udp`
- **GET** `/transit/firewall/udp/rules`
- **POST** `/transit/firewall/udp/rules`
- **DELETE** `/transit/firewall/udp/rules/{ruleId}`

### NAT State Management

#### Export NAT State
- **GET** `/transit/state`
- Exports complete NAT table state
- Includes all NAT mappings and port allocations
- Used for high availability/failover
- Response format:
  ```json
  {
    "ips": ["100.64.0.1", "100.64.0.2"],
    "tcp_inbound": [...],
    "tcp_outbound": [...],
    "udp_inbound": [...],
    "udp_outbound": [...],
    "nat_ports": {...}
  }
  ```

#### Import NAT State
- **PUT** `/transit/state`
- Imports complete NAT table state
- **Atomic operation**: replaces entire state
- Used for failover/migration scenarios
- Request body: Same format as export

### Router Control

#### Outbound NAT Status
- **GET** `/transit/router/nat/outbound`
- Returns outbound NAT enabled/disabled status

#### Enable/Disable Outbound NAT
- **PUT** `/transit/router/nat/outbound`
- Body: `{"enabled": true}`
- Controls outbound NAT functionality

#### Inbound NAT Status
- **GET** `/transit/router/nat/inbound`
- Returns inbound NAT enabled/disabled status

#### Enable/Disable Inbound NAT
- **PUT** `/transit/router/nat/inbound`
- Body: `{"enabled": true}`
- Note: Inbound NAT not currently implemented

#### Interface Routing Status
- **GET** `/transit/router/interfaces`
- Returns interface routing enabled/disabled status

#### Enable/Disable Interface Routing
- **PUT** `/transit/router/interfaces`
- Body: `{"enabled": true}`
- Controls multi-interface support

#### Router Status
- **GET** `/transit/router/status`
- Returns comprehensive router status:
  - Router enabled/disabled
  - Outbound NAT status
  - Inbound NAT status
  - Interface routing status

## OpenAPI Specification

The complete API is defined in `api/rest/v1/openapi.yaml`. This specification:
- Provides detailed request/response schemas
- Includes comprehensive error responses
- Enables automatic client generation
- Serves as the authoritative API reference

### Client Generation

Generate API clients using the OpenAPI specification:

```bash
# Example using openapi-generator
openapi-generator generate \
  -i api/rest/v1/openapi.yaml \
  -g go \
  -o pkg/client
```

## Common Patterns

### Error Responses
- **400**: Bad request (invalid input)
- **404**: Resource not found
- **409**: Conflict (resource already exists)
- **422**: Unprocessable entity (validation failed)
- **500**: Internal server error

### Success Responses
- **200**: Success (GET, PUT, DELETE)
- **201**: Created (POST)

### Request Validation
All requests are validated against the OpenAPI schema before processing, ensuring:
- Required parameters are present
- Data types are correct
- Values meet constraints (e.g., valid IP addresses)

## Usage Examples

### Example: Managing NAT IPs
```bash
# List current IPs
curl -X GET /transit/ips

# Add a new IP
curl -X POST "/transit/ips?ip=100.64.0.1"

# Remove an IP
curl -X DELETE "/transit/ips?ip=100.64.0.1"
```

### Example: Configuring Firewall
```bash
# Enable TCP firewall
curl -X PUT /transit/firewall/tcp \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}'

# Add TCP rule allowing HTTPS
curl -X POST /transit/firewall/tcp/rules \
  -H "Content-Type: application/json" \
  -d '{
    "source_cidr": "0.0.0.0/0",
    "destination_cidr": "10.0.0.0/8",
    "source_port_low": 1024,
    "source_port_high": 65535,
    "destination_port_low": 443,
    "destination_port_high": 443
  }'
```

### Example: DDoS Protection
```bash
# Enable DDoS protection
curl -X PUT /transit/ddos \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}'

# Block an attacking IP
curl -X POST "/transit/ddos/ips?ip=192.168.1.100"

# Reset all blocked IPs
curl -X DELETE /transit/ddos
```

## Integration with Transit

The HTTP API directly manipulates the Transit dataplane components described in the [Transit Implementation Guide](transit-implementation.md). Changes made through the API are applied immediately to the live dataplane without requiring a restart.

The API provides runtime access to all major Transit components:
- Firewall rules and state
- DDoS protection settings
- NAT IP management
- Router configuration
- Event control (via enable/disable flags)