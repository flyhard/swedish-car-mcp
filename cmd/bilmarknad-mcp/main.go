package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/flyhard/swedish-car-mcp/internal/bilmarknad/server"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := server.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
