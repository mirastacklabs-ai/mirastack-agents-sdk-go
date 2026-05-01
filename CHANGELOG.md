# Changelog

All notable changes to `mirastack-agents-sdk-go` are recorded in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Release Cadence — Lockstep With Python SDK

`mirastack-agents-sdk-go` and
[`mirastack-agents-sdk-python`](https://github.com/mirastacklabs-ai/mirastack-agents-sdk-python)
**ship together at matching `MAJOR.MINOR` tags**. Every minor or major bump
in either SDK forces a paired release of the other so plugin authors writing
in either language consume the same engine handshake contract.

Patch (`Z`) versions are independent: a Go-only or Python-only patch fix
(e.g. mypy/ruff lint, build-only fix) MAY ship without bumping the
counterpart SDK as long as the wire contract is unchanged.

When a counterpart SDK has no functional change for a paired minor bump,
the release is a *version-alignment* tag — the package is republished with
the matching version number, the CHANGELOG records the alignment, and the
README links to the counterpart's release notes for the actual feature.

All MIRASTACK agents — Go and Python — MUST be on the latest paired SDK
minor before the engine cuts a release; the engine's CI gate enforces this.

## [1.8.0] — 2026-05-01

### Changed (BREAKING)
- `pluginv1.LicenseQuotas.MaxDataSourceTypes` renamed to
  `MaxIntegrationTypes`. The JSON tag also moves from
  `max_data_source_types` to `max_integration_types`. This mirrors the
  engine's `internal/license` rebrand from "data sources" to
  "integrations". Callers reading the field must be updated; there is no
  compat shim because the engine ships the new tag and the JWT signer
  refuses to mint legacy tokens.

### Migration
1. Rename any `q.MaxDataSourceTypes` reference in your agent to
   `q.MaxIntegrationTypes`.
2. Bump `go.mod`: `github.com/mirastacklabs-ai/mirastack-agents-sdk-go v1.8.0`.
3. `go mod tidy && go build ./...`

## [1.7.1] — 2026-04

### Added
- Lazy tenant-bound registration. Plugins now keep retrying
  `RegisterPlugin` while the engine is in bootstrap mode or the bound
  tenant has not been created yet, instead of crashing on first failure.
  Backoff is exponential, capped at 30 s, and structured-logged with the
  classified failure reason.

## [1.7.0] — 2026-04

### Added
- `pluginv1.LicenseContext` and `pluginv1.LicenseQuotas` exposed on
  `RegisterPluginResponse.License` and `HeartbeatResponse.License`.
  Plugins can read the engine's licensing snapshot at registration time
  and on every heartbeat without polling a separate endpoint.
- When `License.Active` flips to `false` between heartbeats the SDK
  consumer SHOULD stop accepting new `ExecuteRequest`s.

## [1.6.0] — 2026-03

### Added
- Multi-tenant `tenant_id` propagation. Every outbound gRPC call from
  `EngineContext` is auto-stamped with the plugin's bound `tenant_id`.
  `MIRASTACK_PLUGIN_TENANT_SLUG` is the preferred deployment input;
  `MIRASTACK_PLUGIN_TENANT_ID` overrides it. At least one MUST be set —
  the SDK refuses to start without a tenant identity.

## [1.5.0] — 2026-02

### Added
- `EngineContext.CacheGetBatch` (MGET) for batched cache lookups.
- `EngineContext.CallPluginWithTimeRange` so cross-plugin calls preserve
  the ingress anchor time and avoid drift.
- Dedicated `Heartbeat` RPC separate from `RegisterPlugin`.
- gRPC keepalive on both server and `EngineContext` client.

[1.8.0]: https://github.com/mirastacklabs-ai/mirastack-agents-sdk-go/releases/tag/v1.8.0
[1.7.1]: https://github.com/mirastacklabs-ai/mirastack-agents-sdk-go/releases/tag/v1.7.1
[1.7.0]: https://github.com/mirastacklabs-ai/mirastack-agents-sdk-go/releases/tag/v1.7.0
[1.6.0]: https://github.com/mirastacklabs-ai/mirastack-agents-sdk-go/releases/tag/v1.6.0
[1.5.0]: https://github.com/mirastacklabs-ai/mirastack-agents-sdk-go/releases/tag/v1.5.0
