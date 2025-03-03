package main

import (
	"flag"
	"log"
	"runtime"

	"nightmare/internal/logger"
	"nightmare/pkg/config"
	"nightmare/pkg/engine"
)

func init() {
	// GLFW requires the program to be running on the main thread
	runtime.LockOSThread()
}

func main() {
	// Инициализация логгера
	logger := logger.NewLogger("debug")
	logger.Info("Starting Nightmare ASCII Horror Game...")

	// Чтение конфигурации
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Инициализация игрового движка
	game, err := engine.NewEngine(cfg, logger)
	if err != nil {
		log.Fatalf("Failed to initialize game engine: %v", err)
	}

	// Запуск игрового цикла
	logger.Info("Engine initialized, starting game loop...")
	game.Run()
}
