package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/TFMV/surrealcode"
)

func main() {
	var (
		dir          = flag.String("dir", ".", "Directory to scan for Go files")
		outputFormat = flag.String("format", "dot", "Output format for call graph: dot or d3")
		outputFile   = flag.String("out", "", "Output file (if empty, prints to stdout)")
		dbURL        = flag.String("db", "ws://localhost:8000/rpc", "SurrealDB connection URL")
		namespace    = flag.String("namespace", "test", "SurrealDB namespace")
		database     = flag.String("database", "test", "SurrealDB database")
		dbUser       = flag.String("db-user", "root", "SurrealDB username")
		dbPass       = flag.String("db-pass", "root", "SurrealDB password")
	)
	flag.Parse()

	analyzer, err := surrealcode.NewAnalyzer(*dbURL, *namespace, *database, *dbUser, *dbPass)
	if err != nil {
		log.Fatalf("Failed to create analyzer: %v", err)
	}

	if err := analyzer.Initialize(); err != nil {
		log.Fatalf("Failed to initialize analyzer: %v", err)
	}

	visualization, err := analyzer.GenerateVisualization(*dir, *outputFormat)
	if err != nil {
		log.Fatalf("Failed to generate visualization: %v", err)
	}

	if *outputFile != "" {
		if err := os.WriteFile(*outputFile, []byte(visualization), 0644); err != nil {
			log.Fatalf("Failed to write output file: %v", err)
		}
		fmt.Printf("Export written to %s\n", *outputFile)
	} else {
		fmt.Println(visualization)
	}

	if err := analyzer.AnalyzeDirectory(*dir); err != nil {
		log.Fatalf("Failed to analyze directory: %v", err)
	}

	fmt.Println("Code analysis completed successfully!")
}
