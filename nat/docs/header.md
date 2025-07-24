# Architect NAT Terraform Module

Highly-available NAT deployment using Architect's networking solution with sub-second failover capabilities.

## Architecture

This module implements a dual-instance architecture with fast failover:

```
┌─────────────────────────────────────────────────────────────────┐
│                           VPC                                   │
│                                                                 │
│  ┌─────────────────────┐        ┌─────────────────────┐       │
│  │   Private Subnet    │        │  Architect Subnet   │       │
│  │                     │        │    (Dedicated)      │       │
│  │   ┌─────────────┐   │        │                     │       │
│  │   │ Application │   │        │  ┌──────────────┐   │       │
│  │   │  Instance   │───┼────────┼─▶│ ENI Blue     │   │       │
│  │   └─────────────┘   │        │  │ 10.x.x.10    │   │       │
│  │                     │        │  │ 10.x.x.12 ◄──┼───┼───────┤ Floating IP
│  └─────────────────────┘        │  └──────────────┘   │       │
│                                 │                     │       │
│                                 │  ┌──────────────┐   │       │
│                                 │  │ ENI Red      │   │       │
│                                 │  │ 10.x.x.11    │   │       │
│                                 │  └──────────────┘   │       │
│                                 └─────────────────────┘       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                                          │
                                          ▼
                                  Internet Gateway
```

During failover, the floating IP (10.x.x.12) moves from Blue ENI to Red ENI, and route tables are updated accordingly.