package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- m4: help must exit 0, not "flag: help requested" -----------------------

func TestProbeHelpIsNotAnError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"probe", "-h"}, &stdout, &stderr); err != nil {
		t.Fatalf("probe -h must not return an error, got %v", err)
	}
	if !strings.Contains(stderr.String(), "-model") {
		t.Fatalf("probe -h should print the flag usage to stderr, got %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "flag: help requested") {
		t.Fatalf("probe -h must not print the bogus flag error, got %q", stderr.String())
	}
}

func TestModelsHelpIsNotAnError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"models", "-h"}, &stdout, &stderr); err != nil {
		t.Fatalf("models -h must not return an error, got %v", err)
	}
	if strings.Contains(stderr.String(), "flag: help requested") {
		t.Fatalf("models -h must not print the bogus flag error, got %q", stderr.String())
	}
}

func TestProbeUnknownFlagIsStillAnError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"probe", "--bogus"}, &stdout, &stderr); err == nil {
		t.Fatal("unknown flag must stay an error")
	}
}

func TestVersionCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"version"}, &stdout, &stderr); err != nil {
		t.Fatalf("version: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != version {
		t.Fatalf("version printed %q, want %q", got, version)
	}
}

// --- m5: positional arguments must be rejected, not silently ignored -------

func TestProbeRejectsPositionalArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"probe", "qwen3-27b"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("hwcfgmap probe qwen3-27b (missing --model) must be an error, not silent probe-only output")
	}
	if !strings.Contains(err.Error(), "--model qwen3-27b") {
		t.Fatalf("error should suggest --model qwen3-27b, got %q", err.Error())
	}
	if stdout.Len() != 0 {
		t.Fatalf("probe-only output must not be printed on the error path, got %q", stdout.String())
	}
}

func TestModelsRejectsPositionalArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"models", "extra"}, &stdout, &stderr); err == nil {
		t.Fatal("models extra must be an error")
	}
}

func TestProbeLaunchOnlyStillWorks(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"probe", "--model", "qwen3-27b", "--launch-only"}, &stdout, &stderr); err != nil {
		t.Fatalf("launch-only happy path broke: %v", err)
	}
	if !strings.Contains(stdout.String(), "llama-server") {
		t.Fatalf("expected a llama-server line, got %q", stdout.String())
	}
}

// The `hwcfgmap --model x` shorthand (leading flag) still routes to probe.
func TestFlagShorthandRoutesToProbe(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--model", "qwen3-27b", "--launch-only"}, &stdout, &stderr); err != nil {
		t.Fatalf("shorthand invocation broke: %v", err)
	}
	if !strings.Contains(stdout.String(), "llama-server") {
		t.Fatalf("expected a llama-server line, got %q", stdout.String())
	}
}

// --- m8: version lockstep ---------------------------------------------------

// oldVersion is assembled at runtime so this test file never carries the
// literal and trips the repo-wide sweep below.
var oldVersion = string([]byte{'0', '.', '1', '.', '0'})

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// thisFile = <repo>/cmd/hwcfgmap/main_test.go
	return filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestVersionLockstep(t *testing.T) {
	root := repoRoot(t)

	want := "0.2.0"
	if got := strings.TrimSpace(mustRead(t, filepath.Join(root, "VERSION"))); got != want {
		t.Fatalf("VERSION file = %q, want %q", got, want)
	}
	// Dev builds carry the -dev suffix; goreleaser stamps the release version
	// via ldflags (see the version var comment in main.go).
	if base := strings.TrimSuffix(version, "-dev"); base != want {
		t.Fatalf("version constant base = %q, want %q (VERSION file)", base, want)
	}
	for _, name := range []string{"README.md", "README.en.md"} {
		body := mustRead(t, filepath.Join(root, name))
		if !strings.Contains(body, "`v"+want+"`") {
			t.Fatalf("%s must mention `v%s`", name, want)
		}
		if strings.Contains(body, "`v"+oldVersion+"`") {
			t.Fatalf("%s still mentions the previous version `v%s`", name, oldVersion)
		}
	}
	site := mustRead(t, filepath.Join(root, "web", "site.json"))
	if !strings.Contains(site, `"content_version": "`+want+`"`) {
		t.Fatalf("web/site.json meta.content_version must be %q", want)
	}
	changelog := mustRead(t, filepath.Join(root, "CHANGELOG.md"))
	for _, section := range []string{"[" + oldVersion + "]", "[0.2.0]"} {
		if !strings.Contains(changelog, section) {
			t.Fatalf("CHANGELOG.md must contain a %s section", section)
		}
	}
}

// TestNoStaleVersionStrings sweeps the repo: outside the frozen
// initial-release recordings (docs/demo-results.json, assets/demo.gif) and
// the CHANGELOG history, no live file may still carry the previous version
// string.
func TestNoStaleVersionStrings(t *testing.T) {
	root := repoRoot(t)
	skipFiles := map[string]bool{
		"CHANGELOG.md":           true, // release history
		"docs/demo-results.json": true, // frozen initial-release demo recording
		"assets/demo.gif":        true, // frozen initial-release render
	}
	var offenders []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if skipFiles[rel] || strings.HasPrefix(d.Name(), ".build_metadata") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(body), oldVersion) {
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("files still carrying the previous version string: %v", offenders)
	}
}
