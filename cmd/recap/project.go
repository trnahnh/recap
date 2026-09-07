package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/trnahnh/recap/internal/model"
	"github.com/trnahnh/recap/internal/store"
)

type projectFinder interface {
	GetProjectByRoot(ctx context.Context, rootPath string) (model.Project, error)
}

func resolveProject(ctx context.Context, finder projectFinder, override string) (model.Project, error) {
	root := override
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return model.Project{}, fmt.Errorf("resolving working directory: %w", err)
		}
		root = cwd
	}
	project, err := finder.GetProjectByRoot(ctx, root)
	if errors.Is(err, store.ErrNotFound) {
		return model.Project{}, fmt.Errorf(
			"no project registered for %s — run `recap project add` there first", root)
	}
	if err != nil {
		return model.Project{}, err
	}
	return project, nil
}

func runProject(ctx context.Context, args []string, w io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: recap project <add|list> [args]")
	}
	switch args[0] {
	case "add":
		return runProjectAdd(ctx, args[1:], w)
	case "list":
		return runProjectList(ctx, args[1:], w)
	default:
		return fmt.Errorf("unknown project command %q (want add or list)", args[0])
	}
}

func runProjectAdd(ctx context.Context, args []string, w io.Writer) error {
	fs := flag.NewFlagSet("recap project add", flag.ContinueOnError)
	custom := fs.String("config", "", "path to config file (default: OS config dir)")
	name := fs.String("name", "", "project name (default: directory name)")
	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	root := ""
	if len(positional) > 0 {
		root = positional[0]
	}
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("resolving working directory: %w", err)
		}
		root = cwd
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", root, err)
	}
	if *name == "" {
		*name = filepath.Base(abs)
	}

	configPath, err := resolveConfigPath(*custom)
	if err != nil {
		return err
	}
	a, err := openApp(ctx, configPath)
	if err != nil {
		return err
	}
	defer a.close()

	project, err := a.store.CreateProject(ctx, *name, abs)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "project %s registered at %s (%s)\n", project.Name, project.RootPath, project.ID)
	return nil
}

func runProjectList(ctx context.Context, args []string, w io.Writer) error {
	fs := flag.NewFlagSet("recap project list", flag.ContinueOnError)
	custom := fs.String("config", "", "path to config file (default: OS config dir)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	configPath, err := resolveConfigPath(*custom)
	if err != nil {
		return err
	}
	a, err := openApp(ctx, configPath)
	if err != nil {
		return err
	}
	defer a.close()

	projects, err := a.store.ListProjects(ctx)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tROOT")
	for _, p := range projects {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", p.ID, p.Name, p.RootPath)
	}
	return tw.Flush()
}
