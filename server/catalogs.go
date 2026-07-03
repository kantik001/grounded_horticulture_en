package main

import "sync/atomic"

// runtimeCatalogs groups all hot-reloadable JSON configs. The whole set is
// replaced atomically on reload (SIGHUP / polling), so request handlers never
// observe a partially updated or concurrently mutated catalog.
type runtimeCatalogs struct {
	Crops          cropsFile
	Prompts        map[string]cropPrompts
	Onboarding     map[string][]string
	PhotoTemplates map[string]string
	Branding       BrandingConfig
}

var catalogsPtr atomic.Pointer[runtimeCatalogs]

// currentCatalogs returns the active catalog set. Never returns nil: before
// the first successful load (only possible in tests) it returns empty catalogs.
func currentCatalogs() *runtimeCatalogs {
	if c := catalogsPtr.Load(); c != nil {
		return c
	}
	return &runtimeCatalogs{}
}

// loadRuntimeCatalogs parses all JSON configs into a new set and swaps it in
// atomically. On any error the previous set stays active.
func loadRuntimeCatalogs() error {
	crops, err := loadCropCatalog()
	if err != nil {
		return err
	}
	prompts, err := loadPromptCatalog()
	if err != nil {
		return err
	}
	onboarding, err := loadOnboardingConfig()
	if err != nil {
		return err
	}
	photoTemplates, err := loadPhotoTemplates()
	if err != nil {
		return err
	}
	branding, err := loadBrandingConfig()
	if err != nil {
		return err
	}
	catalogsPtr.Store(&runtimeCatalogs{
		Crops:          crops,
		Prompts:        prompts,
		Onboarding:     onboarding,
		PhotoTemplates: photoTemplates,
		Branding:       branding,
	})
	return nil
}
