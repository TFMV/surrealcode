package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/TFMV/surrealcode"
	"github.com/docopt/docopt-go"
)

const usage = `SurrealCode - Go Code Analysis Tool.

Usage:
  surrealcode analyze [--dir=<path>] [--db=<url>] [--namespace=<ns>] [--database=<db>] [--db-user=<user>] [--db-pass=<pass>]
  surrealcode -h | --help
  surrealcode --version

Options:
  -h --help            Show this help message.
  --version            Show version.
  --dir=<path>         Directory to scan for Go files [default: .].
  --db=<url>           SurrealDB connection URL [default: ws://localhost:8000/rpc].
  --namespace=<ns>     SurrealDB namespace [default: test].
  --database=<db>      SurrealDB database [default: test].
  --db-user=<user>     SurrealDB username [default: root].
  --db-pass=<pass>     SurrealDB password [default: root].
`

const version = "0.1.0"

func main() {
	opts, err := docopt.ParseArgs(usage, os.Args[1:], version)
	if err != nil {
		log.Fatalf("Error parsing arguments: %v", err)
	}

	if cmd, _ := opts.Bool("analyze"); cmd {
		dir, _ := opts.String("--dir")
		dbURL, _ := opts.String("--db")
		namespace, _ := opts.String("--namespace")
		database, _ := opts.String("--database")
		dbUser, _ := opts.String("--db-user")
		dbPass, _ := opts.String("--db-pass")

		analyzer, err := surrealcode.NewAnalyzer(dbURL, namespace, database, dbUser, dbPass)
		if err != nil {
			log.Fatalf("Failed to create analyzer: %v", err)
		}

		if err := analyzer.Initialize(); err != nil {
			log.Fatalf("Failed to initialize analyzer: %v", err)
		}

		if err := analyzer.AnalyzeDirectory(context.Background(), dir); err != nil {
			log.Fatalf("Failed to analyze directory: %v", err)
		}

		fmt.Println("Code analysis completed successfully!")
	}
}
