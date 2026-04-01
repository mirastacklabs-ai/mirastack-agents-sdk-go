# MIRASTACK SDK for Go

Go SDK for building **MIRASTACK** plugins. Provides the base interfaces, engine context proxy, and gRPC server bootstrap required to implement a plugin in Go.

## Installation

```bash
go get github.com/mirastacklabs-ai/mirastack-sdk-go
```

## Quick Start

```go
package main

import (
    "context"
    sdk "github.com/mirastacklabs-ai/mirastack-sdk-go"
)

type MyPlugin struct {
    sdk.BasePlugin
}

func (p *MyPlugin) Info() *sdk.PluginInfo {
    return &sdk.PluginInfo{
        Name:        "my-plugin",
        Version:     "0.1.0",
        Permission:  sdk.PermissionRead,
        DevOpsStage: sdk.StageObserve,
    }
}

func (p *MyPlugin) Execute(ctx context.Context, req *sdk.ExecuteRequest) (*sdk.ExecuteResponse, error) {
    // Plugin logic here
    return &sdk.ExecuteResponse{
        Output: map[string]string{"result": "done"},
    }, nil
}

func main() {
    sdk.Serve(&MyPlugin{})
}
```

## SDK Components

| File | Purpose |
|------|---------|
| `plugin.go` | Plugin interface + base struct |
| `context.go` | Engine context proxy (config, cache, events) |
| `serve.go` | gRPC server bootstrap |
| `gen/` | Generated protobuf Go types |

## Plugin Interface

```go
type Plugin interface {
    Info() *PluginInfo
    GetSchema() *Schema
    Execute(ctx context.Context, req *ExecuteRequest) (*ExecuteResponse, error)
    HealthCheck(ctx context.Context) error
    ConfigUpdated(ctx context.Context, config map[string]string) error
}
```

## Engine Context

Plugins can interact with the engine via the context proxy:

```go
// Read configuration
value, err := ctx.GetConfig("victoriametrics.url")

// Cache operations
ctx.CacheSet("key", "value", 5*time.Minute)
value, err := ctx.CacheGet("key")

// Publish results
ctx.PublishResult(result)

// Request approval
approved, err := ctx.RequestApproval("Delete old data?", sdk.PermissionModify)
```

## License

AGPL v3 — see [LICENSE](LICENSE).
