package store

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNormalizeRootPathAbsoluteAndSlashed(t *testing.T) {
	got, err := NormalizeRootPath(".")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if !filepath.IsAbs(filepath.FromSlash(got)) {
		t.Fatalf("expected an absolute path, got %q", got)
	}
	if strings.Contains(got, `\`) {
		t.Fatalf("expected forward slashes only, got %q", got)
	}
}

func TestNormalizeRootPathIdempotent(t *testing.T) {
	once, err := NormalizeRootPath(t.TempDir())
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	twice, err := NormalizeRootPath(once)
	if err != nil {
		t.Fatalf("normalize again: %v", err)
	}
	if once != twice {
		t.Fatalf("not idempotent: %q then %q", once, twice)
	}
}

func TestNormalizeRootPathVariantsCollapse(t *testing.T) {
	dir := t.TempDir()
	base, err := NormalizeRootPath(dir)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	variants := []string{
		dir + string(os.PathSeparator),
		filepath.Join(dir, "sub", ".."),
	}
	if runtime.GOOS == "windows" {
		variants = append(variants, strings.ToUpper(dir), filepath.ToSlash(dir))
	}

	for _, v := range variants {
		got, err := NormalizeRootPath(v)
		if err != nil {
			t.Fatalf("normalize %q: %v", v, err)
		}
		if got != base {
			t.Fatalf("variant %q normalized to %q, want %q", v, got, base)
		}
	}
}

func TestNormalizeRootPathPreservesCaseOffWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows paths are case-folded by design")
	}
	got, err := NormalizeRootPath("/tmp/RecapCase")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if !strings.HasSuffix(got, "/RecapCase") {
		t.Fatalf("expected case to be preserved, got %q", got)
	}
}

func TestNormalizeRootPathRejectsEmpty(t *testing.T) {
	for _, in := range []string{"", "   "} {
		if _, err := NormalizeRootPath(in); err == nil {
			t.Fatalf("expected an error for %q, got nil", in)
		}
	}
}
