package main

import (
	"context"
	"github.com/dnozdrin/websockets/chat/server/http"
	"github.com/dnozdrin/websockets/chat/server/websocket"
	"go.uber.org/zap"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	mainLog, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer func() {
		if err := mainLog.Sync(); err != nil {
			if err.Error() == "sync /dev/stderr: invalid argument" {
				log.Print(err)
			} else {
				log.Fatalf("can't flush buffered log entries: %v", err)
			}
		}
	}()

	logger := mainLog.Sugar()
	ctx, cancel := context.WithCancel(context.Background())
	setupGracefulShutdown(cancel, logger.Error)

	var wg = &sync.WaitGroup{}
	httpServ := http.New(
		http.Config{Port: 8081},
		logger,
	)
	websocketServ := websocket.New(
		websocket.Config{Port: 8088},
		logger,
	)
	httpServ.Run(ctx, wg)
	websocketServ.Run(ctx, wg)

	wg.Wait()
	log.Print("Server stopped")
}

func setupGracefulShutdown(stop func(), log func(...interface{})) {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		log("Got Interrupt signal")
		stop()
	}()
}
