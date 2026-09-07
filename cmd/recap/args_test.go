package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trnahnh/recap/internal/model"
	"github.com/trnahnh/recap/internal/store"
)

func TestParseListFilter(t *testing.T) {
	f, err := parseListFilter("draft, active", "decision")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.statuses) != 2 || f.statuses[0] != model.RecordStatusDraft || f.statuses[1] != model.RecordStatusActive {
		t.Fatalf("statuses parsed as %v", f.statuses)
	}
	if len(f.types) != 1 || f.types[0] != model.RecordTypeDecision {
		t.Fatalf("types parsed as %v", f.types)
	}

	empty, err := parseListFilter("", "")
	if err != nil || len(empty.statuses) != 0 || len(empty.types) != 0 {
		t.Fatalf("empty filter: %+v, %v", empty, err)
	}

	if _, err := parseListFilter("bogus", ""); err == nil || !strings.Contains(err.Error(), "--status") {
		t.Fatalf("expected a --status error, got %v", err)
	}
	if _, err := parseListFilter("", "bogus"); err == nil || !strings.Contains(err.Error(), "--type") {
		t.Fatalf("expected a --type error, got %v", err)
	}
}

func TestRecordIDArg(t *testing.T) {
	if _, err := recordIDArg(nil, "recap show <id>"); err == nil || !strings.HasPrefix(err.Error(), "usage:") {
		t.Fatalf("missing id: expected usage error, got %v", err)
	}
	if _, err := recordIDArg([]string{"not-a-uuid"}, ""); err == nil || !strings.Contains(err.Error(), "invalid record id") {
		t.Fatalf("bad id: expected invalid-id error, got %v", err)
	}
	valid := "0b5d6f1e-2a3b-4c5d-8e9f-0a1b2c3d4e5f"
	id, err := recordIDArg([]string{valid}, "")
	if err != nil || id != valid {
		t.Fatalf("valid id: got %q, %v", id, err)
	}
}

func TestParseInterspersedAcceptsFlagsAfterPositionals(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	project := fs.String("project", "", "")
	title := fs.String("title", "", "")

	positional, err := parseInterspersed(fs, []string{"--title", "a", "some-id", "--project", "/p", "extra"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if *project != "/p" || *title != "a" {
		t.Fatalf("flags after the positional were not parsed: project=%q title=%q", *project, *title)
	}
	if len(positional) != 2 || positional[0] != "some-id" || positional[1] != "extra" {
		t.Fatalf("positionals parsed as %v", positional)
	}

	fs = flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(&strings.Builder{})
	if _, err := parseInterspersed(fs, []string{"id", "--unknown"}); err == nil {
		t.Fatal("expected an unknown flag after the positional to error")
	}
}

func TestEditFlagsApply(t *testing.T) {
	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	var flags editFlags
	flags.register(fs)
	if err := fs.Parse([]string{"--title", "new title", "--rationale", "", "--confidence", "0.5"}); err != nil {
		t.Fatal(err)
	}
	flags.collect(fs)
	if !flags.anyField() {
		t.Fatal("expected field flags to be detected")
	}

	fields := editableFields{Title: "old", Task: "task", Rationale: optionalString("keep?")}
	if err := flags.applyTo(&fields); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if fields.Title != "new title" || fields.Task != "task" {
		t.Fatalf("unexpected fields after apply: %+v", fields)
	}
	if fields.Rationale != nil {
		t.Fatalf("an explicit empty --rationale should clear the field, got %q", *fields.Rationale)
	}
	if fields.Confidence == nil || *fields.Confidence != 0.5 {
		t.Fatalf("confidence not applied: %+v", fields.Confidence)
	}

	fs = flag.NewFlagSet("edit", flag.ContinueOnError)
	flags = editFlags{}
	flags.register(fs)
	if err := fs.Parse([]string{"--confidence", "1.5"}); err != nil {
		t.Fatal(err)
	}
	flags.collect(fs)
	if err := flags.applyTo(&fields); err == nil {
		t.Fatal("expected out-of-range confidence to be rejected")
	}

	fs = flag.NewFlagSet("edit", flag.ContinueOnError)
	flags = editFlags{}
	flags.register(fs)
	fs.String("project", "", "")
	if err := fs.Parse([]string{"--project", "/x"}); err != nil {
		t.Fatal(err)
	}
	flags.collect(fs)
	if flags.anyField() {
		t.Fatal("--project alone must not count as an edit field")
	}
}

type fakeFinder struct {
	projects map[string]model.Project
	asked    []string
}

func (f *fakeFinder) GetProjectByRoot(_ context.Context, rootPath string) (model.Project, error) {
	f.asked = append(f.asked, rootPath)
	root, err := store.NormalizeRootPath(rootPath)
	if err != nil {
		return model.Project{}, err
	}
	if p, ok := f.projects[root]; ok {
		return p, nil
	}
	return model.Project{}, store.ErrNotFound
}

func TestResolveProjectUsesOverrideOrCwd(t *testing.T) {
	dir := t.TempDir()
	root, err := store.NormalizeRootPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := model.Project{ID: "p1", Name: "recap", RootPath: root}
	finder := &fakeFinder{projects: map[string]model.Project{root: want}}

	got, err := resolveProject(context.Background(), finder, dir)
	if err != nil || got.ID != want.ID {
		t.Fatalf("override: got %+v, %v", got, err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	got, err = resolveProject(context.Background(), finder, "")
	if err != nil || got.ID != want.ID {
		t.Fatalf("cwd: got %+v, %v", got, err)
	}
	if len(finder.asked) != 2 {
		t.Fatalf("expected two lookups, got %v", finder.asked)
	}
}

func TestResolveProjectNotRegistered(t *testing.T) {
	finder := &fakeFinder{projects: map[string]model.Project{}}
	unknown := filepath.Join(t.TempDir(), "elsewhere")

	_, err := resolveProject(context.Background(), finder, unknown)
	if err == nil {
		t.Fatal("expected an error for an unregistered project")
	}
	if errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected the raw ErrNotFound to be replaced by guidance, got %v", err)
	}
	if !strings.Contains(err.Error(), "recap project add") || !strings.Contains(err.Error(), unknown) {
		t.Fatalf("error should name the path and the fix, got %q", err)
	}
}
