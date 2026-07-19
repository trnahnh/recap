package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

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
	case "-h", "--help", "help":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown command %q (run `recap help`)", cmd)
	}
}

func withConfigPath(args []string, fn func(path string) error) error {
	fs := flag.NewFlagSet("recap", flag.ContinueOnError)
	custom := fs.String("config", "", "path to config file (default: OS config dir)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	path := *custom
	if path == "" {
		def, err := config.DefaultPath()
		if err != nil {
			return err
		}
		path = def
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

Commands (Phase 1a):
  init      generate config, start Postgres, apply migrations
  start     start the database for an initialized install
  stop      stop the database (data is preserved)
  status    report daemon/database health
  help      show this message

Later phases add: save, list, search, show, edit, delete, archive, export, import.
`)
}
