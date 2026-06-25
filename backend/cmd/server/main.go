package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"werewolf/backend/internal/config"
	"werewolf/backend/internal/game"
	"werewolf/backend/internal/httpapi"
)

func main() {
	runtimePath := config.ResolveRuntimeConfigPath([]string{
		filepath.Clean("config/app.json"),
		filepath.Clean("../config/app.json"),
	})
	runtimeConfig, err := config.LoadRuntimeConfig(runtimePath)
	if err != nil {
		log.Fatalf("load runtime config: %v", err)
	}

	presetsPathSetting := runtimeConfig.Backend.PresetsPath
	if env := os.Getenv("WEREWOLF_PRESETS_PATH"); env != "" {
		presetsPathSetting = env
	}
	presetsPath := config.ResolveRelativeToConfig(runtimePath, presetsPathSetting)
	presets, err := config.LoadPresets(presetsPath)
	if err != nil {
		log.Fatalf("load presets: %v", err)
	}

	service := game.NewService(presets)
	server := httpapi.NewServer(service)
	addr := fmt.Sprintf(":%d", runtimeConfig.Backend.Port)
	log.Printf("werewolf backend listening on %s using %s", addr, presetsPath)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatal(err)
	}
}
