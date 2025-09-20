# DiStore - Distributed Key-Value Store

A lightweight, distributed key-value storage system written in Go that provides high availability through data replication across multiple nodes.

## Features

- **Distributed Storage**: Data is automatically replicated across multiple nodes for fault tolerance
- **Multiple Storage Backends**: Choose between in-memory or persistent disk-based storage
- **HTTP REST API**: Simple JSON-based API for all operations
- **Configurable Replication**: Adjust replication factor based on your availability requirements
- **Health Checks**: Built-in health monitoring endpoints
- **Graceful Shutdown**: Proper handling of shutdown signals for data integrity

## API Endpoints

### Public Endpoints
- `POST /set` - Store a key-value pair
- `GET /get/{key}` - Retrieve a value by key
- `DELETE /delete/{key}` - Remove a key-value pair
- `GET /keys` - Get all stored key-value pairs
- `GET /health` - Health check endpoint

### Internal Endpoints (for replication)
- `POST /internal/set` - Internal replication endpoint for SET operations
- `DELETE /internal/delete/{key}` - Internal replication endpoint for DELETE operations

## Quick Start

### Prerequisites
- Go 1.25 or later

### Installation

1. Clone the repository:
```bash
git clone https://github.com/Julzz10110/DiStore.git
cd DiStore