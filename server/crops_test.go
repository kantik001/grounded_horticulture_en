package main

import (
	"os"
	"path/filepath"
	"testing"
)

func cropsConfigForTest(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		filepath.Join("config", "crops.json"),
		filepath.Join("..", "config", "crops.json"),
	} {
		p := filepath.Join(wd, rel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("config/crops.json not found")
	return ""
}

func TestNormalizeCropID(t *testing.T) {
	t.Setenv("CROPS_CONFIG_PATH", cropsConfigForTest(t))
	cropCatalog = cropsFile{}
	if err := loadCropCatalog(); err != nil {
		t.Fatalf("loadCropCatalog: %v", err)
	}

	id, err := normalizeCropID("apple")
	if err != nil || id != "apple" {
		t.Fatalf("apple: id=%q err=%v", id, err)
	}

	_, err = normalizeCropID("unknown_crop_xyz")
	if err == nil {
		t.Fatal("expected error for unknown crop")
	}
}
