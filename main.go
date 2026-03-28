package main

import (
	"embed"
	"io/fs"
	"log"
	"os"

	"dartcounter/internal/config"
	"dartcounter/internal/server"
	"dartcounter/internal/sound"
	"dartcounter/internal/storage"
)

//go:embed all:web
var webContent embed.FS

func main() {
	cfg := config.Load()

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting DartCounter...")

	// Open database
	db, err := storage.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	log.Println("Database ready")

	// Ensure sounds directory exists with default manifest
	if err := os.MkdirAll(cfg.SoundsDir+"/default", 0755); err != nil {
		log.Printf("Warning: could not create sounds directory: %v", err)
	}
	if _, err := os.Stat(cfg.SoundsDir + "/default/manifest.json"); os.IsNotExist(err) {
		sound.CreateDefaultManifest(cfg.SoundsDir + "/default")
		log.Println("Created default sound manifest")
	}

	// Get embedded web filesystem
	webFS, err := fs.Sub(webContent, "web")
	if err != nil {
		log.Fatalf("Failed to load web content: %v", err)
	}

	// Start server
	srv := server.New(cfg, db, webFS)
	log.Fatal(srv.Start())
}
