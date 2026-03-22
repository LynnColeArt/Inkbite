package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/LynnColeArt/Inkbite"
	"github.com/LynnColeArt/Inkbite/builtins"
	"github.com/LynnColeArt/Inkbite/internal/components"
	"github.com/LynnColeArt/Inkbite/internal/ocr"
)

var version = "dev"

type runtimeDeps struct {
	version        string
	executablePath string
	helperSelfTest func(helperPath string, backend string) error
}

func main() {
	executablePath, _ := os.Executable()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, runtimeDeps{
		version:        version,
		executablePath: executablePath,
		helperSelfTest: helperSelfTest,
	}))
}

func run(args []string, stdout io.Writer, stderr io.Writer, deps runtimeDeps) int {
	if len(args) > 0 {
		switch strings.ToLower(strings.TrimSpace(args[0])) {
		case "convert":
			return runConvert(args[1:], stdout, stderr, deps.version)
		case "components":
			return runComponents(args[1:], stdout, stderr, deps)
		case "install":
			return runInstall(args[1:], stdout, stderr, deps)
		case "doctor":
			return runDoctor(stdout, stderr, deps)
		case "config":
			return runConfig(args[1:], stdout, stderr, deps)
		case "__ocr_helper":
			return runOCRHelper(args[1:], stdout, stderr)
		}
	}

	return runConvert(args, stdout, stderr, deps.version)
}

func runConvert(args []string, stdout io.Writer, stderr io.Writer, version string) int {
	flags := flag.NewFlagSet("convert", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var (
		output       string
		extension    string
		mimeType     string
		charset      string
		keepDataURIs bool
		enableHTTP   bool
		pdfBackend   string
		listFormats  bool
		showVersion  bool
	)

	flags.StringVar(&output, "output", "", "write markdown output to file")
	flags.StringVar(&output, "o", "", "write markdown output to file")
	flags.StringVar(&extension, "extension", "", "file extension hint")
	flags.StringVar(&extension, "x", "", "file extension hint")
	flags.StringVar(&mimeType, "mime-type", "", "MIME type hint")
	flags.StringVar(&mimeType, "m", "", "MIME type hint")
	flags.StringVar(&charset, "charset", "", "charset hint")
	flags.StringVar(&charset, "c", "", "charset hint")
	flags.BoolVar(&keepDataURIs, "keep-data-uris", false, "keep inline data URIs in output")
	flags.BoolVar(&enableHTTP, "http", false, "allow fetching http(s) URIs")
	flags.StringVar(&pdfBackend, "pdf-backend", "auto", "pdf backend selection (auto|purego)")
	flags.BoolVar(&listFormats, "list-formats", false, "list registered converters")
	flags.BoolVar(&showVersion, "version", false, "print version and exit")
	flags.BoolVar(&showVersion, "v", false, "print version and exit")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}

	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)

	if listFormats {
		for _, converter := range engine.RegisteredConverters() {
			fmt.Fprintf(stdout, "%s\t(priority %.0f)\n", converter.Name(), converter.Priority())
		}
		return 0
	}

	info := &inkbite.StreamInfo{
		Extension: extension,
		MIMEType:  mimeType,
		Charset:   charset,
	}
	if info.Extension == "" && info.MIMEType == "" && info.Charset == "" {
		info = nil
	}

	opts := inkbite.ConvertOptions{
		KeepDataURIs: keepDataURIs,
		EnableHTTP:   enableHTTP,
		PDFBackend:   pdfBackend,
	}

	var (
		result inkbite.Result
		err    error
	)

	if flags.NArg() == 0 {
		result, err = engine.ConvertReader(context.Background(), os.Stdin, info, opts)
	} else {
		target := strings.TrimSpace(flags.Arg(0))
		result, err = engine.Convert(context.Background(), target, info, opts)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if output != "" {
		if err := os.WriteFile(output, []byte(result.Markdown), 0o644); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}

	if _, err := io.WriteString(stdout, result.Markdown); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if result.Markdown != "" && !strings.HasSuffix(result.Markdown, "\n") {
		_, _ = io.WriteString(stdout, "\n")
	}

	return 0
}

func runComponents(args []string, stdout io.Writer, stderr io.Writer, deps runtimeDeps) int {
	if len(args) == 0 || strings.EqualFold(args[0], "list") {
		return runComponentsList(stdout, stderr, deps)
	}

	fmt.Fprintln(stderr, "usage: inkbite components list")
	return 1
}

func runComponentsList(stdout io.Writer, stderr io.Writer, deps runtimeDeps) int {
	manager := components.Manager{
		Version: deps.version,
	}

	items, err := manager.ListComponents()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(items) == 0 {
		fmt.Fprintln(stdout, "no managed components installed")
		return 0
	}

	for _, item := range items {
		fmt.Fprintf(stdout, "%s\tbackend=%s\tversion=%s\tpath=%s\n", item.Name, item.Backend, item.Version, item.InstallDir)
	}
	return 0
}

func runInstall(args []string, stdout io.Writer, stderr io.Writer, deps runtimeDeps) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: inkbite install ocr [--backend auto|cpu|cuda|rocm|metal] [--dir path]")
		return 1
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "ocr", "all":
		return runInstallOCR(args[1:], stdout, stderr, deps)
	default:
		fmt.Fprintln(stderr, "usage: inkbite install ocr [--backend auto|cpu|cuda|rocm|metal] [--dir path]")
		return 1
	}
}

func runInstallOCR(args []string, stdout io.Writer, stderr io.Writer, deps runtimeDeps) int {
	flags := flag.NewFlagSet("install ocr", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var (
		backend string
		dir     string
	)

	flags.StringVar(&backend, "backend", "auto", "ocr backend selection (auto|cpu|cuda|rocm|metal)")
	flags.StringVar(&dir, "dir", "", "override component base directory")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	manager := components.Manager{
		BaseDir:        dir,
		Version:        deps.version,
		ExecutablePath: deps.executablePath,
		HelperSelfTest: deps.helperSelfTest,
	}

	component, err := manager.InstallOCR(backend)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	fmt.Fprintln(stdout, "installed managed ocr component")
	fmt.Fprintf(stdout, "backend: %s\n", component.Backend)
	fmt.Fprintf(stdout, "version: %s\n", component.Version)
	fmt.Fprintf(stdout, "path: %s\n", component.InstallDir)
	return 0
}

func runDoctor(stdout io.Writer, stderr io.Writer, deps runtimeDeps) int {
	manager := components.Manager{
		Version:        deps.version,
		HelperSelfTest: deps.helperSelfTest,
	}

	report, err := manager.Doctor()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	fmt.Fprintf(stdout, "base_dir: %s\n", report.BaseDir)
	fmt.Fprintf(stdout, "config_path: %s\n", report.ConfigPath)

	exitCode := 0
	for _, component := range report.Components {
		if !component.Installed {
			fmt.Fprintf(stdout, "%s: not installed\n", component.Name)
			continue
		}

		fmt.Fprintf(stdout, "%s: installed\n", component.Name)
		fmt.Fprintf(stdout, "  backend: %s\n", component.Backend)
		fmt.Fprintf(stdout, "  version: %s\n", component.Version)
		fmt.Fprintf(stdout, "  path: %s\n", component.InstallDir)
		fmt.Fprintf(stdout, "  helper: %s\n", component.HelperPath)
		if len(component.Issues) == 0 {
			fmt.Fprintln(stdout, "  status: ok")
			continue
		}

		exitCode = 1
		fmt.Fprintln(stdout, "  status: issues detected")
		for _, issue := range component.Issues {
			fmt.Fprintf(stdout, "  issue: %s\n", issue)
		}
	}

	return exitCode
}

func runConfig(args []string, stdout io.Writer, stderr io.Writer, deps runtimeDeps) int {
	if len(args) == 0 || strings.EqualFold(args[0], "show") {
		return runConfigShow(stdout, stderr, deps)
	}

	fmt.Fprintln(stderr, "usage: inkbite config show")
	return 1
}

func runConfigShow(stdout io.Writer, stderr io.Writer, deps runtimeDeps) int {
	manager := components.Manager{
		Version: deps.version,
	}

	cfg, _, err := manager.CurrentConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	data = append(data, '\n')
	if _, err := stdout.Write(data); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runOCRHelper(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("__ocr_helper", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var (
		selfTest bool
		backend  string
	)

	flags.BoolVar(&selfTest, "self-test", false, "run OCR helper self-test")
	flags.StringVar(&backend, "backend", "cpu", "ocr backend selection")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if !selfTest {
		fmt.Fprintln(stderr, "usage: inkbite __ocr_helper --self-test [--backend cpu]")
		return 1
	}

	if err := ocr.SelfTest(backend); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	normalized, _ := ocr.ResolveBackend(backend)
	payload := map[string]string{
		"status":  "ok",
		"backend": normalized,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	data = append(data, '\n')
	_, _ = stdout.Write(data)
	return 0
}

func helperSelfTest(helperPath string, backend string) error {
	cmd := exec.Command(helperPath, "__ocr_helper", "--self-test", "--backend", backend)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(string(output))
		}
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("ocr helper self-test failed: %s", message)
	}
	return nil
}
