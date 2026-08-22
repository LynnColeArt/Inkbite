package acceptance_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	inkbite "github.com/LynnColeArt/Inkbite"
	"github.com/LynnColeArt/Inkbite/builtins"
)

func TestEveryIngestionLimitPassesAtBoundaryAndFailsAtPlusOne(t *testing.T) {
	t.Run("source bytes", func(t *testing.T) {
		policy := inkbite.DefaultIngestionPolicy()
		policy.MaxSourceBytes = 8
		assertDetailedBoundary(t, policy, []byte("12345678"), testConversion("ok", nil), true)
		assertDetailedBoundary(t, policy, []byte("123456789"), testConversion("ok", nil), false)
	})
	t.Run("primary bytes", func(t *testing.T) {
		policy := inkbite.DefaultIngestionPolicy()
		policy.MaxPrimaryBytes = 8
		assertDetailedBoundary(t, policy, []byte("source"), testConversion("12345678", nil), true)
		assertDetailedBoundary(t, policy, []byte("source"), testConversion("123456789", nil), false)
	})
	t.Run("artifact count", func(t *testing.T) {
		policy := inkbite.DefaultIngestionPolicy()
		policy.MaxArtifacts = 1
		assertDetailedBoundary(t, policy, []byte("source"), testConversion("ok", []string{"a"}), true)
		assertDetailedBoundary(t, policy, []byte("source"), testConversion("ok", []string{"a", "b"}), false)
	})
	t.Run("artifact bytes", func(t *testing.T) {
		policy := inkbite.DefaultIngestionPolicy()
		policy.MaxArtifactBytes = 5
		assertDetailedBoundary(t, policy, []byte("source"), testConversion("ok", []string{"12345"}), true)
		assertDetailedBoundary(t, policy, []byte("source"), testConversion("ok", []string{"123456"}), false)
	})
	t.Run("total artifact bytes", func(t *testing.T) {
		policy := inkbite.DefaultIngestionPolicy()
		policy.MaxTotalArtifactBytes = 8
		assertDetailedBoundary(t, policy, []byte("source"), testConversion("ok", []string{"1234", "5678"}), true)
		assertDetailedBoundary(t, policy, []byte("source"), testConversion("ok", []string{"1234", "56789"}), false)
	})

	containerCases := []struct {
		name       string
		policy     func() inkbite.IngestionPolicy
		at         func(*testing.T) []byte
		plusOne    func(*testing.T) []byte
		atFilename string
	}{
		{
			name: "container entries",
			policy: func() inkbite.IngestionPolicy {
				policy := inkbite.DefaultIngestionPolicy()
				policy.MaxContainerEntries = 2
				return policy
			},
			at: func(t *testing.T) []byte {
				return makeZIP(t, []zipEntry{{name: "a.txt", body: []byte("a"), method: zip.Store}, {name: "b.txt", body: []byte("b"), method: zip.Store}})
			},
			plusOne: func(t *testing.T) []byte {
				return makeZIP(t, []zipEntry{{name: "a.txt", body: []byte("a"), method: zip.Store}, {name: "b.txt", body: []byte("b"), method: zip.Store}, {name: "c.txt", body: []byte("c"), method: zip.Store}})
			},
		},
		{
			name: "container entry bytes",
			policy: func() inkbite.IngestionPolicy {
				policy := inkbite.DefaultIngestionPolicy()
				policy.MaxContainerEntryBytes = 5
				return policy
			},
			at: func(t *testing.T) []byte {
				return makeZIP(t, []zipEntry{{name: "a.txt", body: []byte("12345"), method: zip.Store}})
			},
			plusOne: func(t *testing.T) []byte {
				return makeZIP(t, []zipEntry{{name: "a.txt", body: []byte("123456"), method: zip.Store}})
			},
		},
		{
			name: "expanded bytes",
			policy: func() inkbite.IngestionPolicy {
				policy := inkbite.DefaultIngestionPolicy()
				policy.MaxExpandedBytes = 5
				return policy
			},
			at: func(t *testing.T) []byte {
				return makeZIP(t, []zipEntry{{name: "a.txt", body: []byte("12345"), method: zip.Store}})
			},
			plusOne: func(t *testing.T) []byte {
				return makeZIP(t, []zipEntry{{name: "a.txt", body: []byte("123456"), method: zip.Store}})
			},
		},
		{
			name: "container depth",
			policy: func() inkbite.IngestionPolicy {
				policy := inkbite.DefaultIngestionPolicy()
				policy.MaxContainerDepth = 2
				return policy
			},
			at: func(t *testing.T) []byte {
				inner := makeZIP(t, []zipEntry{{name: "leaf.txt", body: []byte("leaf"), method: zip.Store}})
				return makeZIP(t, []zipEntry{{name: "inner.zip", body: inner, method: zip.Store}})
			},
			plusOne: func(t *testing.T) []byte {
				inner := makeZIP(t, []zipEntry{{name: "leaf.txt", body: []byte("leaf"), method: zip.Store}})
				middle := makeZIP(t, []zipEntry{{name: "inner.zip", body: inner, method: zip.Store}})
				return makeZIP(t, []zipEntry{{name: "middle.zip", body: middle, method: zip.Store}})
			},
		},
		{
			name: "expansion ratio",
			policy: func() inkbite.IngestionPolicy {
				policy := inkbite.DefaultIngestionPolicy()
				policy.MaxExpansionRatio = 1
				return policy
			},
			at: func(t *testing.T) []byte {
				return makeZIP(t, []zipEntry{{name: "stored.txt", body: bytes.Repeat([]byte("a"), 4096), method: zip.Store}})
			},
			plusOne: func(t *testing.T) []byte {
				return makeZIP(t, []zipEntry{{name: "deflated.txt", body: bytes.Repeat([]byte("a"), 4096), method: zip.Deflate}})
			},
		},
	}
	for _, tc := range containerCases {
		t.Run(tc.name, func(t *testing.T) {
			engine := inkbite.New()
			builtins.RegisterDefaultConverters(engine)
			policy := tc.policy()
			hints := &inkbite.StreamInfo{Filename: "boundary.zip", Extension: ".zip"}
			at, err := engine.Ingest(context.Background(), tc.at(t), hints, inkbite.IngestOptions{Policy: policy})
			if err != nil || !inkbite.VerifyEnvelope(at).Valid {
				t.Fatalf("at boundary result/error = %#v/%v", at, err)
			}
			failed, err := engine.Ingest(context.Background(), tc.plusOne(t), hints, inkbite.IngestOptions{Policy: policy})
			if !errors.Is(err, inkbite.ErrLimitExceeded) || !reflect.DeepEqual(failed, inkbite.IngestionEnvelope{}) {
				t.Fatalf("plus one result/error = %#v/%v, want zero/limit", failed, err)
			}
		})
	}
}

func TestRemoteAuthorityAddressAndRedirectMatrix(t *testing.T) {
	var calls atomic.Int64
	engine := inkbite.New(inkbite.WithHTTPClient(&http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("transport must not run")
	})}))
	for run := 0; run < 100; run++ {
		envelope, err := engine.Ingest(context.Background(), "https://93.184.216.34/disabled.txt", nil, inkbite.IngestOptions{})
		if !errors.Is(err, inkbite.ErrRemoteDisabled) || !reflect.DeepEqual(envelope, inkbite.IngestionEnvelope{}) {
			t.Fatalf("disabled run %d result/error = %#v/%v", run, envelope, err)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("disabled remote issued %d transport calls", calls.Load())
	}

	policy := inkbite.DefaultIngestionPolicy()
	policy.Remote.Enabled = true
	denied := []string{
		"http://127.0.0.1/a.txt",
		"http://10.0.0.1/a.txt",
		"http://169.254.1.1/a.txt",
		"http://0.0.0.0/a.txt",
		"http://[::1]/a.txt",
		"http://[fc00::1]/a.txt",
		"http://[fe80::1]/a.txt",
		"http://[::]/a.txt",
	}
	for _, rawURL := range denied {
		envelope, err := engine.Ingest(context.Background(), rawURL, nil, inkbite.IngestOptions{Policy: policy})
		if !errors.Is(err, inkbite.ErrPolicyViolation) || !reflect.DeepEqual(envelope, inkbite.IngestionEnvelope{}) {
			t.Errorf("%s result/error = %#v/%v, want zero/policy", rawURL, envelope, err)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("denied addresses issued %d transport calls", calls.Load())
	}

	redirectCalls := atomic.Int64{}
	redirectEngine := inkbite.New(inkbite.WithHTTPClient(&http.Client{Transport: roundTripper(func(request *http.Request) (*http.Response, error) {
		redirectCalls.Add(1)
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"http://127.0.0.1/secret.txt"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    request,
		}, nil
	})}))
	envelope, err := redirectEngine.Ingest(context.Background(), "http://93.184.216.34/start.txt", nil, inkbite.IngestOptions{Policy: policy})
	if !errors.Is(err, inkbite.ErrPolicyViolation) || !reflect.DeepEqual(envelope, inkbite.IngestionEnvelope{}) {
		t.Fatalf("redirect result/error = %#v/%v, want zero/policy", envelope, err)
	}
	if redirectCalls.Load() != 1 {
		t.Fatalf("redirect transport calls = %d, want one admitted hop", redirectCalls.Load())
	}
}

func TestConversionHasZeroHiddenModelComponentOrDownloadEffects(t *testing.T) {
	sentinelDir := t.TempDir()
	counters := map[string]string{
		"model":     filepath.Join(sentinelDir, "model.count"),
		"component": filepath.Join(sentinelDir, "component.count"),
		"download":  filepath.Join(sentinelDir, "download.count"),
	}
	commands := map[string]string{
		"ollama":    counters["model"],
		"paddleocr": counters["component"],
		"curl":      counters["download"],
		"wget":      counters["download"],
	}
	for command, counter := range commands {
		writeInvocationSentinel(t, sentinelDir, command, counter)
	}
	t.Setenv("PATH", sentinelDir)

	var transportCalls atomic.Int64
	engine := inkbite.New(inkbite.WithHTTPClient(&http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
		transportCalls.Add(1)
		return nil, errors.New("unexpected transport")
	})}))
	builtins.RegisterDefaultConverters(engine)
	for _, format := range canonicalFormats(t) {
		_ = ingestCanonical(t, engine, format)
	}
	if transportCalls.Load() != 0 {
		t.Fatalf("local conversion issued %d transport calls", transportCalls.Load())
	}
	for effect, counter := range counters {
		if data, err := os.ReadFile(counter); err == nil {
			t.Errorf("hidden %s effect invoked: %q", effect, data)
		} else if !os.IsNotExist(err) {
			t.Error(err)
		}
	}
}

func TestCancellationReturnsTypedZeroEnvelopeAcrossOneHundredRequests(t *testing.T) {
	for run := 0; run < 100; run++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		reader := &countingReader{data: []byte("must not read")}
		envelope, err := inkbite.New().Ingest(ctx, reader, nil, inkbite.IngestOptions{})
		if !errors.Is(err, inkbite.ErrCancellation) || !errors.Is(err, context.Canceled) || !reflect.DeepEqual(envelope, inkbite.IngestionEnvelope{}) {
			t.Fatalf("run %d result/error = %#v/%v", run, envelope, err)
		}
		if reader.calls.Load() != 0 {
			t.Fatalf("run %d read calls = %d, want zero", run, reader.calls.Load())
		}
	}
}

func TestFailureDiagnosticsRedactSecrets(t *testing.T) {
	const secret = "SENSITIVE-WP09-SECRET"
	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "remote credentials",
			run: func() error {
				_, err := inkbite.New().Ingest(context.Background(), "https://user:"+secret+"@example.invalid/a.txt", nil, inkbite.IngestOptions{})
				return err
			},
		},
		{
			name: "data URI payload",
			run: func() error {
				_, err := inkbite.New().Ingest(context.Background(), "data:text/plain,%G0"+secret, nil, inkbite.IngestOptions{})
				return err
			},
		},
		{
			name: "converter detail",
			run: func() error {
				engine := inkbite.New()
				engine.RegisterConverter(&acceptanceConverter{failure: errors.New(secret)})
				_, err := engine.Ingest(context.Background(), []byte("source"), nil, inkbite.IngestOptions{})
				return err
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if err == nil || strings.Contains(err.Error(), secret) {
				t.Fatalf("unsafe diagnostic: %v", err)
			}
		})
	}
}

func TestPackageReproducibilityCleanupTrapDoesNotEscapeFunction(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "scripts", "verify-ingestion-contract.sh")
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	definitions, _, found := strings.Cut(string(script), "\ncase \"${1:-quality}\" in")
	if !found {
		t.Fatal("release script dispatch marker not found")
	}
	probe := definitions + `
build_packages() {
  mkdir -p "$1"
  printf 'package' >"$1/archive"
  printf 'hash  archive\n' >"$1/checksums.txt"
}
sha256_file() { printf 'hash\n'; }
caller_frame() { verify_package_reproducibility; }
caller_frame
`
	command := exec.Command("bash")
	command.Dir = filepath.Join("..", "..")
	command.Stdin = strings.NewReader(probe)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("reproducibility cleanup escaped its function: %v\n%s", err, output)
	}
}

func TestQualityCleanlinessAllowsOnlyUnchangedPreexistingReviewLock(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "scripts", "verify-ingestion-contract.sh")
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	definitions, _, found := strings.Cut(string(script), "\ncase \"${1:-quality}\" in")
	if !found {
		t.Fatal("release script dispatch marker not found")
	}

	tests := []struct {
		name           string
		baselineStatus string
		finalStatus    string
		initialLock    string
		mutation       string
		wantSuccess    bool
	}{
		{
			name:           "unchanged review lock",
			baselineStatus: "?? .spec-kitty/review-lock.json",
			finalStatus:    "?? .spec-kitty/review-lock.json",
			initialLock:    `{"owner":"review"}`,
			mutation:       "none",
			wantSuccess:    true,
		},
		{
			name:        "created review lock",
			finalStatus: "?? .spec-kitty/review-lock.json",
			mutation:    "create-lock",
		},
		{
			name:           "deleted review lock",
			baselineStatus: "?? .spec-kitty/review-lock.json",
			initialLock:    `{"owner":"review"}`,
			mutation:       "delete-lock",
		},
		{
			name:           "modified review lock",
			baselineStatus: "?? .spec-kitty/review-lock.json",
			finalStatus:    "?? .spec-kitty/review-lock.json",
			initialLock:    `{"owner":"review"}`,
			mutation:       "modify-lock",
		},
		{
			name:           "additional spec kitty file",
			baselineStatus: "?? .spec-kitty/review-lock.json",
			finalStatus:    "?? .spec-kitty/extra.json\n?? .spec-kitty/review-lock.json",
			initialLock:    `{"owner":"review"}`,
			mutation:       "create-extra",
		},
		{
			name:           "preexisting unrelated dirt",
			baselineStatus: "?? unrelated.tmp",
			finalStatus:    "?? unrelated.tmp",
			mutation:       "none",
		},
		{
			name:           "preexisting tracked dirt",
			baselineStatus: " M tracked.go",
			finalStatus:    " M tracked.go",
			mutation:       "none",
		},
		{
			name:        "new unrelated dirt",
			finalStatus: "?? unexpected.tmp",
			mutation:    "none",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			worktree := t.TempDir()
			if tc.initialLock != "" {
				if err := os.MkdirAll(filepath.Join(worktree, ".spec-kitty"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(worktree, ".spec-kitty", "review-lock.json"), []byte(tc.initialLock), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.MkdirAll(filepath.Join(worktree, "scripts"), 0o755); err != nil {
				t.Fatal(err)
			}
			mutationScript := `#!/usr/bin/env bash
set -euo pipefail
case "${QUALITY_MUTATION}" in
  none) ;;
  create-lock) mkdir -p .spec-kitty; printf 'created' >.spec-kitty/review-lock.json ;;
  delete-lock) rm -f .spec-kitty/review-lock.json ;;
  modify-lock) printf 'modified' >.spec-kitty/review-lock.json ;;
  create-extra) printf 'extra' >.spec-kitty/extra.json ;;
  *) exit 2 ;;
esac
`
			if err := os.WriteFile(filepath.Join(worktree, "scripts", "changed-coverage.sh"), []byte(mutationScript), 0o700); err != nil {
				t.Fatal(err)
			}

			probe := definitions + `
go() { [[ "${1:-}" != version ]] || printf 'go version go1.26.6 linux/amd64\n'; }
staticcheck() { [[ "${1:-}" != -version ]] || printf 'staticcheck test\n'; }
govulncheck() { [[ "${1:-}" != -version ]] || printf 'govulncheck test\n'; }
gofmt() { :; }
verify_license_inventory() { :; }
verify_release_surfaces() { :; }
verify_public_api() { :; }
verify_autocrlf_fixture() { :; }
verify_package_reproducibility() { :; }
git() {
  case "${1:-}" in
    --version) printf 'git version test\n' ;;
    rev-parse) printf '%s\n' "$IMMUTABLE_BASE" ;;
    ls-files) ;;
    status)
      local count=0
      [[ ! -f .status-count ]] || read -r count <.status-count
      if (( count == 0 )); then
        printf '%s' "${BASELINE_STATUS:-}"
      else
        printf '%s' "${FINAL_STATUS:-}"
      fi
      printf '%s\n' "$((count + 1))" >.status-count
      ;;
    *) ;;
  esac
}
run_quality
`
			command := exec.Command("bash")
			command.Dir = worktree
			command.Stdin = strings.NewReader(probe)
			command.Env = append(os.Environ(),
				"BASELINE_STATUS="+tc.baselineStatus,
				"FINAL_STATUS="+tc.finalStatus,
				"QUALITY_MUTATION="+tc.mutation,
			)
			output, err := command.CombinedOutput()
			if tc.wantSuccess && err != nil {
				t.Fatalf("unchanged review lock was rejected: %v\n%s", err, output)
			}
			if !tc.wantSuccess && err == nil {
				t.Fatalf("dirty quality result was accepted:\n%s", output)
			}
		})
	}
}

func TestSourceOnlyReleaseContractAndArchiveMutations(t *testing.T) {
	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dist := t.TempDir()
	runReleaseScript(t, repository, "package", "contract", "inkbite", dist)

	wantFiles := []string{"checksums.txt", "inkbite_contract_source.tar.gz", "inkbite_contract_source.zip"}
	entries, err := os.ReadDir(dist)
	if err != nil {
		t.Fatal(err)
	}
	var gotFiles []string
	for _, entry := range entries {
		gotFiles = append(gotFiles, entry.Name())
	}
	if !reflect.DeepEqual(gotFiles, wantFiles) {
		t.Fatalf("release files = %v, want source-only %v", gotFiles, wantFiles)
	}
	runReleaseScript(t, repository, "verify-source-package", "contract", "inkbite", dist)

	mutations := []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{name: "linked binary", mutate: func(t *testing.T, mutated string) {
			addZIPEntry(t, mutated, "inkbite_contract_source/inkbite", []byte("\x7fELF linked binary"))
		}},
		{name: "vendored dependency source", mutate: func(t *testing.T, mutated string) {
			addZIPEntry(t, mutated, "inkbite_contract_source/vendor/dependency.go", []byte("package dependency"))
		}},
		{name: "missing required file", mutate: func(t *testing.T, mutated string) {
			command := exec.Command("zip", "-qd", filepath.Join(mutated, "inkbite_contract_source.zip"), "inkbite_contract_source/README.md")
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("remove required archive entry: %v\n%s", err, output)
			}
		}},
		{name: "extra entry", mutate: func(t *testing.T, mutated string) {
			addZIPEntry(t, mutated, "inkbite_contract_source/EXTRA", []byte("extra"))
		}},
		{name: "nondeterministic metadata", mutate: func(t *testing.T, mutated string) {
			readme, err := os.ReadFile(filepath.Join(repository, "README.md"))
			if err != nil {
				t.Fatal(err)
			}
			addZIPEntry(t, mutated, "inkbite_contract_source/README.md", readme)
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			mutated := t.TempDir()
			copyReleaseFiles(t, dist, mutated)
			mutation.mutate(t, mutated)
			writeReleaseChecksums(t, mutated)
			command := exec.Command(filepath.Join(repository, "scripts", "verify-ingestion-contract.sh"), "verify-source-package", "contract", "inkbite", mutated)
			command.Dir = repository
			if output, err := command.CombinedOutput(); err == nil {
				t.Fatalf("release mutation was accepted:\n%s", output)
			}
		})
	}
}

func TestSourceOnlyPublicationSurfacesAndMutations(t *testing.T) {
	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	runReleaseScript(t, repository, "release-surfaces", repository)

	mutations := []struct {
		name    string
		path    string
		replace func(string) string
	}{
		{
			name: "legacy builder divergence",
			path: "scripts/dist.sh",
			replace: func(string) string {
				return "#!/usr/bin/env bash\ngo build ./cmd/inkbite\n"
			},
		},
		{
			name: "broad CI upload glob",
			path: ".github/workflows/ci.yml",
			replace: func(source string) string {
				return strings.Replace(source, "dist/inkbite_ci_source.tar.gz", "dist/*", 1)
			},
		},
		{
			name: "broad tag upload glob",
			path: ".github/workflows/release.yml",
			replace: func(source string) string {
				return strings.Replace(source, "dist/inkbite_${{ github.ref_name }}_source.tar.gz", "dist/*.tar.gz", 1)
			},
		},
		{
			name: "GPL warning deletion",
			path: "README.md",
			replace: func(source string) string {
				return strings.Replace(source, "Default Inkbite binaries link GPL-3.0-only xlsReader, are not MIT-only, and are not qualified for redistribution by this workflow.\n", "", 1)
			},
		},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			fixture := t.TempDir()
			for _, path := range []string{"scripts/dist.sh", ".github/workflows/ci.yml", ".github/workflows/release.yml", "README.md", "ADOPTED_COMPONENTS.md", "CHANGELOG.md"} {
				source, err := os.ReadFile(filepath.Join(repository, path))
				if err != nil {
					t.Fatal(err)
				}
				if path == mutation.path {
					source = []byte(mutation.replace(string(source)))
				}
				target := filepath.Join(fixture, path)
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, source, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.Command(filepath.Join(repository, "scripts", "verify-ingestion-contract.sh"), "release-surfaces", fixture)
			command.Dir = repository
			if output, err := command.CombinedOutput(); err == nil {
				t.Fatalf("publication mutation was accepted:\n%s", output)
			}
		})
	}
}

func runReleaseScript(t *testing.T, repository string, arguments ...string) {
	t.Helper()
	command := exec.Command(filepath.Join(repository, "scripts", "verify-ingestion-contract.sh"), arguments...)
	command.Dir = repository
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("release command %v: %v\n%s", arguments, err, output)
	}
}

func copyReleaseFiles(t *testing.T, source, target string) {
	t.Helper()
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		contents, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(target, entry.Name()), contents, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func addZIPEntry(t *testing.T, dist, name string, contents []byte) {
	t.Helper()
	stage := t.TempDir()
	path := filepath.Join(stage, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("zip", "-q", filepath.Join(dist, "inkbite_contract_source.zip"), filepath.ToSlash(name))
	command.Dir = stage
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("add archive entry: %v\n%s", err, output)
	}
}

func writeReleaseChecksums(t *testing.T, dist string) {
	t.Helper()
	var manifest strings.Builder
	for _, name := range []string{"inkbite_contract_source.tar.gz", "inkbite_contract_source.zip"} {
		contents, err := os.ReadFile(filepath.Join(dist, name))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&manifest, "%x  %s\n", sha256.Sum256(contents), name)
	}
	if err := os.WriteFile(filepath.Join(dist, "checksums.txt"), []byte(manifest.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}

type acceptanceConverter struct {
	conversion inkbite.DetailedConversion
	failure    error
}

func (*acceptanceConverter) Name() string      { return "wp09-acceptance" }
func (*acceptanceConverter) Priority() float64 { return 1 }
func (*acceptanceConverter) Accepts(context.Context, io.ReadSeeker, inkbite.StreamInfo, inkbite.ConvertOptions) bool {
	return true
}
func (converter *acceptanceConverter) Convert(context.Context, io.ReadSeeker, inkbite.StreamInfo, inkbite.ConvertOptions) (inkbite.Result, error) {
	return converter.conversion.Result, converter.failure
}
func (converter *acceptanceConverter) ConvertDetailed(context.Context, io.ReadSeeker, inkbite.StreamInfo, inkbite.ConvertOptions, inkbite.IngestionPolicy) (inkbite.DetailedConversion, error) {
	return converter.conversion, converter.failure
}

func testConversion(markdown string, artifacts []string) inkbite.DetailedConversion {
	conversion := inkbite.DetailedConversion{Result: inkbite.Result{Markdown: markdown}}
	for _, artifact := range artifacts {
		conversion.Artifacts = append(conversion.Artifacts, inkbite.DetailedArtifact{
			Role:       inkbite.ArtifactRoleEmbeddedImage,
			Bytes:      []byte(artifact),
			MediaType:  "image/png",
			Attributes: []inkbite.MetadataFact{},
		})
	}
	return conversion
}

func assertDetailedBoundary(t *testing.T, policy inkbite.IngestionPolicy, source []byte, conversion inkbite.DetailedConversion, wantSuccess bool) {
	t.Helper()
	engine := inkbite.New()
	engine.RegisterConverter(&acceptanceConverter{conversion: conversion})
	envelope, err := engine.Ingest(context.Background(), source, nil, inkbite.IngestOptions{Policy: policy})
	if wantSuccess {
		if err != nil || !inkbite.VerifyEnvelope(envelope).Valid {
			t.Fatalf("at boundary result/error = %#v/%v", envelope, err)
		}
		return
	}
	if !errors.Is(err, inkbite.ErrLimitExceeded) || !reflect.DeepEqual(envelope, inkbite.IngestionEnvelope{}) {
		t.Fatalf("plus one result/error = %#v/%v, want zero/limit", envelope, err)
	}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (roundTrip roundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

type countingReader struct {
	data  []byte
	calls atomic.Int64
}

func (reader *countingReader) Read(buffer []byte) (int, error) {
	reader.calls.Add(1)
	if len(reader.data) == 0 {
		return 0, io.EOF
	}
	read := copy(buffer, reader.data)
	reader.data = reader.data[read:]
	return read, nil
}

func writeInvocationSentinel(t *testing.T, dir, command, counter string) {
	t.Helper()
	name := command
	body := fmt.Sprintf("#!/bin/sh\nprintf invoked > %q\n", counter)
	if runtime.GOOS == "windows" {
		name += ".bat"
		body = fmt.Sprintf("@echo invoked>%s\r\n", counter)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
}
