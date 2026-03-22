package ocr

import (
	"fmt"
	"strings"
)

// Supported OCR modes for future converter integration.
const (
	ModeOff    = "off"
	ModeAuto   = "auto"
	ModeImages = "images"
	ModeForce  = "force"
)

// ResolveBackend normalizes an OCR backend request.
func ResolveBackend(requested string) (string, error) {
	requested = strings.ToLower(strings.TrimSpace(requested))
	switch requested {
	case "", "auto", "cpu":
		return "cpu", nil
	case "cuda", "rocm", "metal":
		return "", fmt.Errorf("ocr backend %q is not yet available", requested)
	default:
		return "", fmt.Errorf("unknown ocr backend %q", requested)
	}
}

// SelfTest validates that the requested OCR backend is usable in this build.
func SelfTest(requested string) error {
	backend, err := ResolveBackend(requested)
	if err != nil {
		return err
	}
	if backend != "cpu" {
		return fmt.Errorf("ocr backend %q is not yet available", backend)
	}
	return nil
}
