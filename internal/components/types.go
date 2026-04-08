package components

import (
	"io"
	"time"
)

// Manager coordinates optional component install state.
type Manager struct {
	BaseDir        string
	Version        string
	ExecutablePath string
	Now            func() time.Time
	HelperSelfTest func(helperPath string, provider string, backend string) error
	ProgressWriter io.Writer
}

// Config captures installed optional component state.
type Config struct {
	OCR *OCRConfig `json:"ocr,omitempty"`
}

// OCRConfig stores the installed OCR component selection.
type OCRConfig struct {
	Enabled    bool   `json:"enabled"`
	Provider   string `json:"provider"`
	Backend    string `json:"backend"`
	Component  string `json:"component"`
	Version    string `json:"version"`
	InstallDir string `json:"install_dir"`
	LastDoctor string `json:"last_doctor,omitempty"`
}

// BundleManifest describes an installed component bundle.
type BundleManifest struct {
	Component    string             `json:"component"`
	Provider     string             `json:"provider"`
	BundleID     string             `json:"bundle_id"`
	Version      string             `json:"version"`
	OS           string             `json:"os"`
	Arch         string             `json:"arch"`
	Backend      string             `json:"backend"`
	Requirements BundleRequirements `json:"requirements"`
	Files        []BundleFile       `json:"files"`
}

// BundleRequirements captures runtime requirements for a bundle.
type BundleRequirements struct {
	MinVRAMMB int `json:"min_vram_mb"`
}

// BundleFile describes a file inside a bundle.
type BundleFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// InstalledComponent is a high-level installed component summary.
type InstalledComponent struct {
	Name       string
	Provider   string
	Backend    string
	Version    string
	InstallDir string
}

// DoctorReport captures doctor output for installed components.
type DoctorReport struct {
	BaseDir    string
	ConfigPath string
	Components []DoctorComponent
}

// DoctorComponent captures per-component validation details.
type DoctorComponent struct {
	Name       string
	Installed  bool
	Provider   string
	Backend    string
	Version    string
	InstallDir string
	HelperPath string
	Issues     []string
}

// Healthy reports whether the component is free of doctor issues.
func (c DoctorComponent) Healthy() bool {
	return len(c.Issues) == 0
}
