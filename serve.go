package mirastack

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Serve starts the plugin gRPC server and blocks until shutdown.
// This is the main entry point for plugin binaries.
//
//	func main() {
//	    mirastack.Serve(&MyPlugin{})
//	}
func Serve(plugin Plugin) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	info := plugin.Info()
	if info == nil {
		logger.Fatal("plugin.Info() must not return nil")
	}

	listenAddr := os.Getenv("MIRASTACK_PLUGIN_ADDR")
	if listenAddr == "" {
		listenAddr = ":0" // OS-assigned port
	}

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logger.Fatal("failed to listen", zap.Error(err))
	}

	server := grpc.NewServer()

	// TODO(phase-2): Register PluginService implementation that delegates to `plugin`
	_ = plugin

	// Write the actual port to stdout for the engine to discover
	addr := lis.Addr().(*net.TCPAddr)
	fmt.Fprintf(os.Stdout, "MIRASTACK_PLUGIN_PORT=%d\n", addr.Port)

	logger.Info("plugin serving",
		zap.String("name", info.Name),
		zap.String("version", info.Version),
		zap.String("addr", lis.Addr().String()),
	)

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Info("shutting down plugin")
		server.GracefulStop()
	}()

	if err := server.Serve(lis); err != nil {
		logger.Fatal("gRPC serve error", zap.Error(err))
	}
}
