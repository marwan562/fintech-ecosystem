package main

import (
	"log"
	"net/http"
	"os"

	"github.com/sapliy/sapliy-core/internal/dashboard"
	"github.com/sapliy/sapliy-core/pkg/jsonutil"
)

func main() {
	pluginDir := os.Getenv("PLUGIN_DIR")
	if pluginDir == "" {
		pluginDir = "./examples/plugins"
	}

	registry := dashboard.NewRegistry(pluginDir)
	if err := registry.LoadPlugins(); err != nil {
		log.Printf("Warning: failed to load plugins: %v", err)
	}

	http.HandleFunc("/plugins", func(w http.ResponseWriter, r *http.Request) {
		jsonutil.WriteJSON(w, http.StatusOK, registry.ListPlugins())
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		jsonutil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	log.Println("Dashboard Service starting on :8085")
	if err := http.ListenAndServe(":8085", nil); err != nil {
		log.Fatal(err)
	}
}
