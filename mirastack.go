// Package mirastack provides the Go SDK for building MIRASTACK plugins.
//
// A plugin implements the Plugin interface and is started via Serve().
//
//	type MyPlugin struct{}
//
//	func (p *MyPlugin) Info() *pluginv1.InfoResponse { ... }
//	func (p *MyPlugin) Schema() *pluginv1.SchemaResponse { ... }
//	func (p *MyPlugin) Execute(ctx context.Context, req *pluginv1.ExecuteRequest) (*pluginv1.ExecuteResponse, error) { ... }
//	func (p *MyPlugin) HealthCheck(ctx context.Context) error { ... }
//	func (p *MyPlugin) ConfigUpdated(ctx context.Context, config map[string]string) error { ... }
//
//	func main() {
//	    mirastack.Serve(&MyPlugin{})
//	}
package mirastack

import "fmt"

const SDKVersion = "0.1.0"

func init() {
	_ = fmt.Sprintf("mirastack-sdk-go/%s", SDKVersion)
}
