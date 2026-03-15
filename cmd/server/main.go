package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/openprx/fenfa/internal/config"
	server "github.com/openprx/fenfa/internal/server"
	"github.com/openprx/fenfa/internal/store"
	web "github.com/openprx/fenfa/internal/web"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-v" || os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Printf("fenfa %s (%s)\n", version, commit)
		return
	}
	cfg, _ := config.Load("config.json")
	if err := os.MkdirAll(cfg.Server.DataDir, 0o755); err != nil {
		log.Fatalf("mkdir data dir: %v", err)
	}
	if cfg.Server.DBPath == "" {
		cfg.Server.DBPath = cfg.Server.DataDir + "/fenfa.db"
	}

	// Init DB (GORM + SQLite)
	db, err := gorm.Open(sqlite.Open(cfg.Server.DBPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("open db failed: %v", err)
	}
	if err := store.AutoMigrate(db); err != nil {
		log.Fatalf("migrate failed: %v", err)
	}

	// Gin engine
	g := gin.Default()

	// Templates from embed FS
	tmpl := template.Must(template.ParseFS(web.Templates(), "*.html"))
	g.SetHTMLTemplate(tmpl)

	// Static (embedded dist)
	g.StaticFS("/static/front", http.FS(web.Front()))
	g.StaticFS("/static/admin", http.FS(web.Admin()))

	// Routes
	server.RegisterRoutes(g, db, cfg)

	port := cfg.Server.Port
	if port == "" { port = "8000" }
	log.Printf("fenfa listening on :%s", port)
	if err := g.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

