// Package main is the entrypoint for aurelianprm.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/astromechza/aurelianprm/internal/dal"
	"github.com/astromechza/aurelianprm/internal/db"
	"github.com/astromechza/aurelianprm/internal/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dbPath := flag.String("db", "aurelianprm.db", "path to SQLite database file")
	addr := flag.String("addr", ":8080", "HTTP listen address")
	flag.Parse()

	sqlDB, err := db.Open(*dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer sqlDB.Close() //nolint:errcheck

	srv, err := web.NewServer(dal.New(sqlDB))
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	fmt.Fprintf(os.Stderr, "listening on %s\n", *addr)
	return http.ListenAndServe(*addr, srv.Handler())
}
