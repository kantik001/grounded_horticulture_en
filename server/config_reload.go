package main

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// reloadRuntimeConfig reloads JSON configs without restarting the process.
// All catalogs are swapped in a single atomic operation (see catalogs.go),
// so concurrent request handlers never see a partially reloaded state.
func reloadRuntimeConfig() error {
	if err := loadRuntimeCatalogs(); err != nil {
		return err
	}
	log.Printf("Config reloaded: crops=%d", len(currentCatalogs().Crops.Crops))
	return nil
}

// startConfigReloadWatcher handles SIGHUP and optional polling (CONFIG_RELOAD_INTERVAL_SEC).
func startConfigReloadWatcher() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP)
	go func() {
		for range sigCh {
			if err := reloadRuntimeConfig(); err != nil {
				log.Printf("SIGHUP config reload failed: %v", err)
			}
		}
	}()

	sec, _ := strconv.Atoi(os.Getenv("CONFIG_RELOAD_INTERVAL_SEC"))
	if sec <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Duration(sec) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := reloadRuntimeConfig(); err != nil {
				log.Printf("periodic config reload failed: %v", err)
			}
		}
	}()
}
