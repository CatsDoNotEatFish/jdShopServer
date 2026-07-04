package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jdShopServer/config"
	"jdShopServer/internal/repository"
	"jdShopServer/internal/router"
)

var Version = "1.0.0"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			runMigrate()
			return
		case "serve":
			runServe()
			return
		case "version":
			fmt.Println("jdShopServer", Version)
			return
		}
	}

	// Default: serve
	runServe()
}

func loadConfig() *config.Config {
	cfgPath := "config.yaml"
	if s := os.Getenv("CONFIG_PATH"); s != "" {
		cfgPath = s
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	return cfg
}

func runMigrate() {
	cfg := loadConfig()
	db, err := repository.Open(cfg.Database.Path, 1)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := repository.RunMigrations(db, "migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	fmt.Println("Migrations completed successfully.")
}

func runServe() {
	cfg := loadConfig()

	db, err := repository.Open(cfg.Database.Path, cfg.Database.MaxOpenConns)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Auto-run migrations on startup
	if err := repository.RunMigrations(db, "migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Database migrations completed.")

	r := router.New(cfg, db, Version)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		srv.Close()
	}()

	log.Printf("jdShopServer %s listening on %s", Version, addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
	log.Println("Server stopped.")
}
