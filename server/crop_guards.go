package main

import "fmt"

// cropDisplayName returns the English crop label when set, otherwise Russian.
func cropDisplayName(c CropInfo) string {
	if c.NameEN != "" {
		return c.NameEN
	}
	return c.NameRU
}

// requireRAGEnabled checks rag_enabled for the crop.
func requireRAGEnabled(cropID string) error {
	info, ok := cropInfo(cropID)
	if !ok {
		return fmt.Errorf("unknown crop: %s", cropID)
	}
	if !info.RAGEnabled {
		return fmt.Errorf("text assistant is not available yet for crop «%s»", cropDisplayName(info))
	}
	return nil
}

// requireCVEnabled checks cv_enabled for the crop.
func requireCVEnabled(cropID string) error {
	info, ok := cropInfo(cropID)
	if !ok {
		return fmt.Errorf("unknown crop: %s", cropID)
	}
	if !info.CVEnabled {
		return fmt.Errorf("photo recognition is not available yet for crop «%s»", cropDisplayName(info))
	}
	return nil
}
