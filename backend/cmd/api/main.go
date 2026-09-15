package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/priyanshjhaa/Sprout/backend/internal/httpapi"
)

const serviceName = "sprout-api"
const defaultAddress = "127.0.0.1:8080"

func main() {
	os.Exit(realMain())
}

func realMain() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return run(ctx, defaultAddress, os.Stdout, os.Stderr)
}

func run(ctx context.Context, address string, stdout, stderr io.Writer) int {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Fprintf(stderr, "%s: listen: %v\n", serviceName, err)
		return 1
	}

	server := httpapi.NewServer(listener.Addr().String())
	fmt.Fprintf(stdout, "%s: listening on http://%s\n", serviceName, listener.Addr())

	if err := httpapi.Serve(ctx, listener, server); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", serviceName, err)
		return 1
	}

	return 0
}
