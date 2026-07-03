package main

import (
	"os"
	"path/filepath"
	"testing"
)

// cropsConfigForTest locates config/crops.json or skips the test.
func cropsConfigForTest(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	candidates := []string{
		filepath.Join(wd, "..", "config", "crops.json"),
		filepath.Join(wd, "config", "crops.json"),
	}
	if env := os.Getenv("CROPS_CONFIG_PATH"); env != "" {
		candidates = append([]string{env}, candidates...)
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("config/crops.json not found")
	return ""
}

// Verifies that normalizeCropID accepts known crops and rejects unknown ones.
func TestNormalizeCropID(t *testing.T) {
	t.Setenv("CROPS_CONFIG_PATH", cropsConfigForTest(t))
	crops, err := loadCropCatalog()
	if err != nil {
		t.Fatalf("loadCropCatalog: %v", err)
	}
	old := catalogsPtr.Load()
	catalogsPtr.Store(&runtimeCatalogs{Crops: crops})
	defer catalogsPtr.Store(old)

	id, err := normalizeCropID("apple")
	if err != nil || id != "apple" {
		t.Fatalf("apple: id=%q err=%v", id, err)
	}

	_, err = normalizeCropID("unknown_crop_xyz")
	if err == nil {
		t.Fatal("expected error for unknown crop")
	}
}
