package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/trnahnh/recap/internal/config"
	"github.com/trnahnh/recap/internal/daemon"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "recap: "+err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}

	cmd, rest := args[0], args[1:]

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	switch cmd {
	case "init":
		return withConfigPath(rest, func(path string) error {
			if err := daemon.Init(ctx, path); err != nil {
				return err
			}
			fmt.Println("recap initialized and database ready.")
			return nil
		})
	case "start":
		return withConfigPath(rest, func(path string) error {
			if err := daemon.Start(ctx, path); err != nil {
				return err
			}
			fmt.Println("recap database started.")
			return nil
		})
	case "stop":
		return withConfigPath(rest, func(path string) error {
			if err := daemon.Stop(ctx, path); err != nil {
				return err
			}
			fmt.Println("recap database stopped.")
			return nil
		})
	case "status":
		return withConfigPath(rest, func(path string) error {
			return printStatus(ctx, path)
		})
	case "export":
		return runExport(ctx, rest)
	case "import":
		return runImport(ctx, rest)
	case "-h", "--help", "help":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown command %q (run `recap help`)", cmd)
	}
}

func runExport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("recap export", flag.ContinueOnError)
	custom := fs.String("config", "", "path to config file (default: OS config dir)")
	out := fs.String("out", "", "output file (default: ./recap-export-<timestamp>.dump)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	path, err := resolveConfigPath(*custom)
	if err != nil {
		return err
	}
	outPath := *out
	if outPath == "" {
		outPath = fmt.Sprintf("recap-export-%s.dump", time.Now().UTC().Format("20060102T150405Z"))
	}
	if err := daemon.Export(ctx, path, outPath); err != nil {
		return err
	}
	fmt.Printf("exported to %s\n", outPath)
	return nil
}

func runImport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("recap import", flag.ContinueOnError)
	custom := fs.String("config", "", "path to config file (default: OS config dir)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: recap import <file> [--config <path>]")
	}
	inPath := fs.Arg(0)
	path, err := resolveConfigPath(*custom)
	if err != nil {
		return err
	}
	if err := daemon.Import(ctx, path, inPath); err != nil {
		return err
	}
	fmt.Printf("imported from %s\n", inPath)
	return nil
}

func resolveConfigPath(custom string) (string, error) {
	if custom != "" {
		return custom, nil
	}
	return config.DefaultPath()
}

func withConfigPath(args []string, fn func(path string) error) error {
	fs := flag.NewFlagSet("recap", flag.ContinueOnError)
	custom := fs.String("config", "", "path to config file (default: OS config dir)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	path, err := resolveConfigPath(*custom)
	if err != nil {
		return err
	}
	return fn(path)
}

func printStatus(ctx context.Context, path string) error {
	r, err := daemon.Status(ctx, path)
	if err != nil {
		return err
	}
	fmt.Printf("config:   %s\n", r.ConfigPath)
	fmt.Printf("database: postgres://%s:%d/%s\n", r.Host, r.Port, r.Database)
	if r.Ready {
		fmt.Println("status:   ready")
	} else {
		fmt.Println("status:   not ready")
		if r.Detail != "" {
			fmt.Printf("detail:   %s\n", r.Detail)
		}
	}
	return nil
}

func usage() {
	fmt.Print(`recap — local memory for AI coding tools

Usage:
  recap <command> [--config <path>]

Commands:
  init      generate config, start Postgres, apply migrations
  start     start the database for an initialized install
  stop      stop the database (data is preserved)
  status    report daemon/database health
  export    back up all data via pg_dump (--out <file>)
  import    restore data from a dump via pg_restore (import <file>)
  help      show this message

Later phases add: save, list, search, show, edit, delete, archive.
`)
}
