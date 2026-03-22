package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/LynnColeArt/Inkbite"
	"github.com/LynnColeArt/Inkbite/builtins"
)

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
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

	flag.StringVar(&output, "output", "", "write markdown output to file")
	flag.StringVar(&output, "o", "", "write markdown output to file")
	flag.StringVar(&extension, "extension", "", "file extension hint")
	flag.StringVar(&extension, "x", "", "file extension hint")
	flag.StringVar(&mimeType, "mime-type", "", "MIME type hint")
	flag.StringVar(&mimeType, "m", "", "MIME type hint")
	flag.StringVar(&charset, "charset", "", "charset hint")
	flag.StringVar(&charset, "c", "", "charset hint")
	flag.BoolVar(&keepDataURIs, "keep-data-uris", false, "keep inline data URIs in output")
	flag.BoolVar(&enableHTTP, "http", false, "allow fetching http(s) URIs")
	flag.StringVar(&pdfBackend, "pdf-backend", "auto", "pdf backend selection (auto|purego)")
	flag.BoolVar(&listFormats, "list-formats", false, "list registered converters")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&showVersion, "v", false, "print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		return 0
	}

	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)

	if listFormats {
		for _, converter := range engine.RegisteredConverters() {
			fmt.Printf("%s\t(priority %.0f)\n", converter.Name(), converter.Priority())
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

	if flag.NArg() == 0 {
		result, err = engine.ConvertReader(context.Background(), os.Stdin, info, opts)
	} else {
		target := strings.TrimSpace(flag.Arg(0))
		result, err = engine.Convert(context.Background(), target, info, opts)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if output != "" {
		if err := os.WriteFile(output, []byte(result.Markdown), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if _, err := os.Stdout.WriteString(result.Markdown); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if result.Markdown != "" && !strings.HasSuffix(result.Markdown, "\n") {
		_, _ = os.Stdout.WriteString("\n")
	}

	return 0
}
