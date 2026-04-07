# mirastack-agents-sdk-go

Go SDK for building **MIRASTACK agents** — the external gRPC plugins that perform READ, MODIFY, and ADMIN actions on platform engineering systems. Agents are pure compute: they receive params via gRPC, connect to external backends, and optionally call engine APIs via the `EngineContext` proxy.

**License:** GNU AGPL v3 — see [LICENSE](LICENSE).

## Installation

```bash
go get github.com/mirastacklabs-ai/mirastack-agents-sdk-go
```

## Quick Start

```go
package main

import (
    "context"
    sdk "github.com/mirastacklabs-ai/mirastack-agents-sdk-go"
)

type MyAgent struct{}

func (a *MyAgent) Info() sdk.PluginInfo {
    return sdk.PluginInfo{
        Name:        "my-agent",
        Version:     "0.1.0",
        Description: "Example observability agent",
        Actions: []sdk.Action{
            {
                ID:          "query",
                Description: "Query metrics for a service",
                Permission:  sdk.PermissionRead,
                Stages:      []string{sdk.StageObserve},
                InputParams: sdk.ParamSchemas{
                    {Name: "service", Type: "string", Required: true},
                },
            },
        },
        Intents: []sdk.IntentPattern{
            {Pattern: `query.*metrics|show.*metrics`, Description: "Query metrics for a service", Priority: 1},
        },
    }
}

func (a *MyAgent) Schema() sdk.PluginSchema {
    return sdk.PluginSchema{Actions: a.Info().Actions}
}

func (a *MyAgent) Execute(ctx context.Context, req *sdk.ExecuteRequest) (*sdk.ExecuteResponse, error) {
    service, _ := req.Params["service"]
    return sdk.RespondMap(map[string]any{"service": service, "status": "ok"}), nil
}

func (a *MyAgent) HealthCheck(ctx context.Context) error { return nil }

func (a *MyAgent) ConfigUpdated(ctx context.Context, cfg map[string]string) error { return nil }

func main() {
    sdk.Serve(&MyAgent{})
}
```

## Agent Interface

```go
type Plugin interface {
    Info() PluginInfo
    Schema() PluginSchema
    Execute(ctx context.Context, req *ExecuteRequest) (*ExecuteResponse, error)
    HealthCheck(ctx context.Context) error
    ConfigUpdated(ctx context.Context, config map[string]string) error
}
```

## Response Helpers

```go
return sdk.RespondMap(map[string]any{"metric": 42.0, "service": "api"}), nil  // typed map
return sdk.RespondJSON(myStruct), nil                                           // any serialisable type
return sdk.RespondError("backend unavailable"), nil                             // error response
return sdk.RespondRaw(jsonBytes), nil                                           // raw JSON passthrough
```

## Agent-Specific Features

### Actions — Tool Catalog Registration

```go
sdk.Action{
    ID:          "restart_service",
    Description: "Restart a Kubernetes deployment",
    Permission:  sdk.PermissionModify,   // READ | MODIFY | ADMIN
    Stages:      []string{sdk.StageOperate},
    InputParams: sdk.ParamSchemas{
        {Name: "namespace",  Type: "string", Required: true},
        {Name: "deployment", Type: "string", Required: true},
    },
    OutputParams: sdk.ParamSchemas{
        {Name: "status", Type: "string"},
    },
}
```

### Intent Patterns — Natural Language Routing

```go
sdk.IntentPattern{
    Pattern:     `restart.*deployment|rollout.*restart`,
    Description: "Restart a Kubernetes deployment",
    Priority:    10,
}
```

### Prompt Templates

```go
sdk.PromptTemplate{
    Name:        "my_agent_analysis",
    Description: "Analysis prompt contributed to the engine's PromptTemplate Store",
    Content:     "Analyse the following data: {{.Data}}",
}
```

## Engine Context

Implement `EngineAware` to receive the `EngineContext` proxy:

```go
type MyAgent struct{ eng *sdk.EngineContext }

func (a *MyAgent) SetEngineContext(e *sdk.EngineContext) { a.eng = e }

// Inside Execute:
url, _  := a.eng.GetConfig("backend.url")
a.eng.CacheSet("key", "value", 5*time.Minute)
val, _  := a.eng.CacheGet("key")
a.eng.PublishResult(ctx, payload)
ok, _   := a.eng.RequestApproval(ctx, "Proceed?", sdk.PermissionModify)
a.eng.LogEvent(ctx, "action_completed", map[string]any{"action": "restart"})
```

## DateTime Utilities

Convert `req.TimeRange` to backend-specific formats — never parse time in a plugin:

```go
start := sdk.FormatEpochSeconds(req.TimeRange.StartEpochMs)  // VictoriaMetrics
start := sdk.FormatEpochMicros(req.TimeRange.StartEpochMs)   // VictoriaTraces
start := sdk.FormatRFC3339(req.TimeRange.StartEpochMs)       // VictoriaLogs
```

## SDK Components

| File | Purpose |
|------|---------| 
| `plugin.go` | `Plugin` interface, `PluginInfo`, `Action`, `IntentPattern`, `PromptTemplate` |
| `context.go` | `EngineContext` proxy — config, cache, publish, approval, audit log |
| `respond.go` | `RespondMap`, `RespondJSON`, `RespondError`, `RespondRaw` helpers |
| `serve.go` | gRPC server bootstrap — call `Serve(agent)` from `main()` |
| `datetimeutils/` | Time format converters for all MIRASTACK backends |
| `gen/pluginv1/` | Generated protobuf Go types |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MIRASTACK_ENGINE_ADDR` | `localhost:50051` | Engine gRPC address |
| `MIRASTACK_PLUGIN_PORT` | `50052` | Port this agent listens on |
