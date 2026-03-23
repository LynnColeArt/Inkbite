package components

import (
	"errors"
	"os"
)

// ListComponents returns the currently configured installed components.
func (m Manager) ListComponents() ([]InstalledComponent, error) {
	baseDir, err := m.baseDir()
	if err != nil {
		return nil, err
	}

	cfg, err := LoadConfig(ConfigPath(baseDir))
	if err != nil {
		return nil, err
	}

	var out []InstalledComponent
	if cfg.OCR != nil && cfg.OCR.Enabled {
		out = append(out, InstalledComponent{
			Name:       "ocr",
			Provider:   cfg.OCR.Provider,
			Backend:    cfg.OCR.Backend,
			Version:    cfg.OCR.Version,
			InstallDir: cfg.OCR.InstallDir,
		})
	}
	return out, nil
}

// CurrentConfig returns the current component config.
func (m Manager) CurrentConfig() (Config, string, error) {
	baseDir, err := m.baseDir()
	if err != nil {
		return Config{}, "", err
	}

	path := ConfigPath(baseDir)
	cfg, err := LoadConfig(path)
	return cfg, path, err
}

// Doctor inspects installed component state.
func (m Manager) Doctor() (DoctorReport, error) {
	baseDir, err := m.baseDir()
	if err != nil {
		return DoctorReport{}, err
	}

	cfgPath := ConfigPath(baseDir)
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return DoctorReport{}, err
	}

	report := DoctorReport{
		BaseDir:    baseDir,
		ConfigPath: cfgPath,
	}

	if cfg.OCR == nil || !cfg.OCR.Enabled {
		report.Components = append(report.Components, DoctorComponent{
			Name:      "ocr",
			Installed: false,
		})
		return report, nil
	}

	component := DoctorComponent{
		Name:       "ocr",
		Installed:  true,
		Provider:   cfg.OCR.Provider,
		Backend:    cfg.OCR.Backend,
		Version:    cfg.OCR.Version,
		InstallDir: cfg.OCR.InstallDir,
		HelperPath: OCRHelperPath(cfg.OCR.InstallDir),
	}

	if _, err := os.Stat(OCRManifestPath(cfg.OCR.InstallDir)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			component.Issues = append(component.Issues, "manifest is missing")
		} else {
			component.Issues = append(component.Issues, err.Error())
		}
	}
	if _, err := os.Stat(component.HelperPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			component.Issues = append(component.Issues, "helper binary is missing")
		} else {
			component.Issues = append(component.Issues, err.Error())
		}
	}

	if len(component.Issues) == 0 {
		if err := m.selfTest(component.HelperPath, component.Provider, component.Backend); err != nil {
			component.Issues = append(component.Issues, "self-test failed: "+err.Error())
		}
	}

	if cfg.OCR != nil && len(component.Issues) == 0 {
		cfg.OCR.LastDoctor = m.now().Format(timeLayout)
		if err := SaveConfig(cfgPath, cfg); err != nil {
			component.Issues = append(component.Issues, "failed to update config: "+err.Error())
		}
	}

	report.Components = append(report.Components, component)
	return report, nil
}

const timeLayout = "2006-01-02T15:04:05Z07:00"
