# Metadata RPC Methods

This document describes the metadata-related RPC methods available in Rollkit.

## Overview

Rollkit stores various metadata in its store to track the state of different operations. These metadata entries can be queried through RPC methods or REST endpoints.

## Available Metadata Keys

| Key | Description |
|-----|-------------|
| `d` | DA included height - the height of the data availability layer that has been included |
| `l` | Last batch data - the last batch data submitted to the data availability layer |
| `last-submitted-header-height` | Last submitted header height - the height of the last header submitted to DA |
| `last-submitted-data-height` | Last submitted data height - the height of the last data submitted to DA |

## RPC Methods

### GetMetadata

Returns metadata for a specific key (existing method).

**Request:**
```protobuf
message GetMetadataRequest {
  string key = 1;
}
```

**Response:**
```protobuf
message GetMetadataResponse {
  bytes value = 1;
}
```

**Example Usage:**
```go
value, err := client.GetMetadata(ctx, "d")
```

### ListMetadataKeys

Returns all available metadata keys with their descriptions.

**Request:**
```protobuf
google.protobuf.Empty
```

**Response:**
```protobuf
message ListMetadataKeysResponse {
  repeated MetadataKey keys = 1;
}

message MetadataKey {
  string key = 1;
  string description = 2;
}
```

**Example Usage:**
```go
keys, err := client.ListMetadataKeys(ctx)
for _, key := range keys {
    fmt.Printf("Key: %s, Description: %s\n", key.Key, key.Description)
}
```

### GetAllMetadata

Returns all available metadata in a single call.

**Request:**
```protobuf
google.protobuf.Empty
```

**Response:**
```protobuf
message GetAllMetadataResponse {
  repeated MetadataEntry metadata = 1;
}

message MetadataEntry {
  string key = 1;
  bytes value = 2;
}
```

**Example Usage:**
```go
metadata, err := client.GetAllMetadata(ctx)
for _, entry := range metadata {
    fmt.Printf("Key: %s, Value: %v\n", entry.Key, entry.Value)
}
```

## REST Endpoints

For convenience, REST endpoints are also available:

### GET /api/v1/metadata/keys

Returns all available metadata keys with descriptions in JSON format.

**Example Response:**
```json
{
  "keys": [
    {
      "key": "d",
      "description": "DA included height - the height of the data availability layer that has been included"
    },
    {
      "key": "l", 
      "description": "Last batch data - the last batch data submitted to the data availability layer"
    }
  ]
}
```

### GET /api/v1/metadata

Returns information about available metadata and RPC method to use.

**Example Response:**
```json
{
  "message": "Use the RPC interface or gRPC-Web to fetch actual metadata values",
  "available_keys": ["d", "l", "last-submitted-header-height", "last-submitted-data-height"],
  "rpc_method": "rollkit.v1.StoreService/GetAllMetadata"
}
```

## Use Cases

1. **Discovery**: Use `ListMetadataKeys` to discover what metadata is available
2. **Monitoring**: Use `GetAllMetadata` to get a snapshot of all node metadata
3. **Specific Queries**: Use `GetMetadata` when you need a specific piece of metadata
4. **Integration**: Use REST endpoints for simple HTTP-based integrations

## Implementation Notes

- All metadata keys are centralized in the `types` package for consistency
- The methods gracefully handle missing metadata keys
- REST endpoints provide convenient HTTP access for web applications
- All methods maintain backward compatibility with existing code