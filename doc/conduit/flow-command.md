# Flow Command Documentation

## Overview

The `flow` command is a powerful debugging and testing tool that allows you to process packet capture (pcap) files through Conduit's eBPF dataplane. It simulates how packets would be processed in a real deployment, making it invaluable for:
- Testing firewall rules
- Debugging NAT behavior
- Validating DDoS protection
- Understanding packet processing decisions

## Usage

```bash
conduit flow -i input.pcap --interface-mac aa:bb:cc:dd:ee:ff --default-destination-mac 11:22:33:44:55:66
```

### Required Parameters

- **`-i, --input`**: Path to the input pcap file containing packets to process
- **`--interface-mac`**: MAC address of the simulated network interface (format: `aa:bb:cc:dd:ee:ff`)
- **`--default-destination-mac`**: Default destination MAC address for processed packets

### Optional Parameters

- **`-o, --output`**: Path to output pcap file for processed packets (if not specified, results are printed to stdout)
- **`--show-counters`**: Display counter values for each packet (useful for debugging drop reasons)

## How It Works

1. **Packet Reading**: Reads packets from the input pcap file sequentially
2. **eBPF Processing**: Each packet is processed through the configured Conduit dataplane
3. **Action Recording**: The XDP action (PASS, DROP, TX, etc.) is recorded for each packet
4. **Output Generation**: 
   - If output file specified: Writes processed packets with action metadata
   - If no output file: Prints human-readable results to stdout

## Configuration

The flow command uses the Transit configuration from your main Conduit config file if available. You can configure:
- Firewall rules
- DDoS protection settings
- NAT IPs and router settings
- Event monitoring preferences

### Example Configuration

```yaml
transit_config:
  interface_mac: "aa:bb:cc:dd:ee:ff"
  management_ip: "50.0.0.1"
  default_destination_mac: "11:22:33:44:55:66"
  
  # Enable components for testing
  disable_router: false
  disable_outbound_nat: false
  disable_tcp_firewall: false
  
  # Add initial rules
  initial_tcp_rules:
    - source_cidr: "10.0.0.0/24"
      destination_cidr: "0.0.0.0/0"
      destination_port_low: 80
      destination_port_high: 80
  
  # Configure NAT
  initial_nat_ips:
    - "100.64.0.1"
    - "100.64.0.2"
```

## Output Format

### Console Output (Default)

When no output file is specified, the flow command prints detailed information for each packet:

```
Packet #1:
  Type: TCP
  Source: 10.0.0.50:45678 -> 8.8.8.8:443
  Action: XDP_TX (Transmitted)
  NAT: 10.0.0.50:45678 -> 100.64.0.1:2048
  Processing Time: 125μs

Packet #2:
  Type: UDP
  Source: 192.168.1.100:53 -> 10.0.0.50:45678
  Action: XDP_DROP
  Drop Reason: Firewall rule violation
  Counter: dropped_udp_firewall
```

### PCAP Output

When an output file is specified:
- Processed packets are written to the pcap file
- XDP action is stored in packet metadata
- Original packet timing is preserved
- Modified packets (e.g., after NAT) show the transformed state

## Use Cases

### 1. Testing Firewall Rules

Create a pcap with various packet types and test your firewall configuration:

```bash
# Generate test packets with different sources/destinations
tcpdump -w test_firewall.pcap -c 100

# Test firewall processing
conduit flow -i test_firewall.pcap \
  --interface-mac 02:00:00:00:00:01 \
  --default-destination-mac 02:00:00:00:00:02
```

### 2. Debugging NAT Behavior

Capture real traffic and understand how NAT processes it:

```bash
# Capture outbound traffic
tcpdump -i eth0 -w outbound.pcap 'dst net not 10.0.0.0/8'

# Process through NAT
conduit flow -i outbound.pcap \
  --interface-mac 02:00:00:00:00:01 \
  --default-destination-mac 02:00:00:00:00:02 \
  -o natted.pcap

# Analyze NAT translations
tcpdump -r natted.pcap -nn
```

### 3. Validating DDoS Protection

Test DDoS rules against attack traffic:

```bash
# Configure DDoS protection in config
# Add attacking IPs to blocklist

# Process attack traffic
conduit flow -i ddos_attack.pcap \
  --interface-mac 02:00:00:00:00:01 \
  --default-destination-mac 02:00:00:00:00:02 \
  --show-counters | grep dropped_ddos
```

### 4. Performance Testing

Process large pcap files to understand performance characteristics:

```bash
# Time processing of large pcap
time conduit flow -i large_capture.pcap \
  --interface-mac 02:00:00:00:00:01 \
  --default-destination-mac 02:00:00:00:00:02 \
  -o /dev/null
```

## Advanced Usage

### Chaining with Other Tools

The flow command works well with standard packet manipulation tools:

```bash
# Filter specific traffic before processing
tcpdump -r input.pcap -w filtered.pcap 'tcp port 443'
conduit flow -i filtered.pcap --interface-mac ... --default-destination-mac ...

# Replay processed packets
conduit flow -i input.pcap ... -o processed.pcap
tcpreplay -i eth0 processed.pcap
```

### Debugging Specific Scenarios

#### Check Why Packets Are Dropped
```bash
conduit flow -i traffic.pcap \
  --interface-mac 02:00:00:00:00:01 \
  --default-destination-mac 02:00:00:00:00:02 \
  --show-counters | grep "XDP_DROP" -A 2
```

#### Verify NAT Port Allocation
```bash
conduit flow -i many_connections.pcap ... | \
  grep "NAT:" | \
  awk '{print $4}' | \
  sort | uniq -c | \
  sort -rn
```

#### Test Management IP Bypass
```bash
# Create pcap with traffic to management IP
conduit flow -i mgmt_traffic.pcap ... | \
  grep "50.0.0.1" | \
  grep -c "XDP_PASS"
```

## Integration with Development Workflow

The flow command is particularly useful during development:

1. **Rule Development**: Test new firewall rules before deployment
2. **Regression Testing**: Ensure changes don't break existing behavior
3. **Performance Optimization**: Identify processing bottlenecks
4. **Documentation**: Create pcaps that demonstrate specific behaviors

### Example Development Workflow

```bash
# 1. Capture baseline traffic
tcpdump -w baseline.pcap -c 1000

# 2. Process with current configuration
conduit flow -i baseline.pcap ... -o before.pcap

# 3. Make configuration changes

# 4. Process with new configuration
conduit flow -i baseline.pcap ... -o after.pcap

# 5. Compare results
diff <(tcpdump -r before.pcap -nn) <(tcpdump -r after.pcap -nn)
```

## Limitations

- **Read-only**: The flow command only simulates packet processing; it doesn't modify the actual dataplane state
- **Single-threaded**: Processes packets sequentially, which may not reflect real concurrent processing
- **No state persistence**: Each run starts with fresh state; connection tracking doesn't persist between packets unless they're in the same session

## Best Practices

1. **Use representative traffic**: Test with pcaps that reflect your actual network patterns
2. **Start simple**: Begin with small, focused pcaps before processing large captures
3. **Validate configurations**: Always test configuration changes with flow before deploying
4. **Save test cases**: Build a library of pcaps for different scenarios
5. **Monitor performance**: Use time measurements to ensure processing efficiency