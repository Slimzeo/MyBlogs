package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"myblog/config"
	"myblog/internal/cache"
	"myblog/internal/db"
	"myblog/internal/service"
	"myblog/internal/web"
)

func main() {
	runtimeConfig := config.Load()
	database, err := db.Open(runtimeConfig)
	if err != nil {
		log.Fatalf("initialize database: %v", err)
	}
	applicationCache := cache.New()
	defer applicationCache.Close()

	services := service.New(database, applicationCache, runtimeConfig)
	webServer, err := web.NewServer(runtimeConfig, services, "templates")
	if err != nil {
		log.Fatalf("initialize web server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              ":" + runtimeConfig.Port,
		Handler:           webServer.Router("static"),
		ReadTimeout:       runtimeConfig.ReadTimeout,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      runtimeConfig.WriteTimeout,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	hitFlushTicker := time.NewTicker(5 * time.Second)
	defer hitFlushTicker.Stop()
	go func() {
		for range hitFlushTicker.C {
			webServer.FlushHits()
		}
	}()

	serverError := make(chan error, 1)
	go func() {
		log.Printf("blog server listening on http://127.0.0.1:%s", runtimeConfig.Port)
		serverError <- httpServer.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case signalValue := <-stop:
		log.Printf("received signal %s, shutting down", signalValue)
	case err := <-serverError:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve http: %v", err)
		}
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), runtimeConfig.ShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(shutdownContext); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	hitFlushTicker.Stop()
	webServer.Close()
	sqlDB, err := database.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}
