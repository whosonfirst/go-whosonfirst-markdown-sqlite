package main

import (
	"context"
	"flag"
	"fmt"
	wof_index "github.com/whosonfirst/go-whosonfirst-index"
	"github.com/whosonfirst/go-whosonfirst-log"
	"github.com/whosonfirst/go-whosonfirst-markdown"
	"github.com/whosonfirst/go-whosonfirst-markdown/parser"
	"github.com/whosonfirst/go-whosonfirst-sqlite"
	"github.com/whosonfirst/go-whosonfirst-sqlite-markdown/tables"
	"github.com/whosonfirst/go-whosonfirst-sqlite/database"
	"github.com/whosonfirst/go-whosonfirst-sqlite/index"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {

	valid_modes := strings.Join(wof_index.Modes(), ",")
	desc_modes := fmt.Sprintf("The mode to use importing data. Valid modes are: %s.", valid_modes)

	mode := flag.String("mode", "directory", desc_modes)

	driver := flag.String("driver", "sqlite3", "")
	var dsn = flag.String("dsn", "index.db", "")

	var input = flag.String("input", "index.md", "What you expect the input Markdown file to be called")

	all := flag.Bool("all", false, "Index all tables")
	documents := flag.Bool("documents", false, "Index the 'documents' table")
	authors := flag.Bool("authors", false, "Index the 'documents_authors' table")
	links := flag.Bool("links", false, "Index the 'documents_links' table")
	search := flag.Bool("search", false, "Index the 'documents_search' table")

	live_hard := flag.Bool("live-hard-die-fast", false, "Enable various performance-related pragmas at the expense of possible (unlikely) database corruption")
	timings := flag.Bool("timings", false, "Display timings during and after indexing")
	var procs = flag.Int("processes", (runtime.NumCPU() * 2), "The number of concurrent processes to index data with")

	flag.Parse()

	runtime.GOMAXPROCS(*procs)

	logger := log.SimpleWOFLogger()

	stdout := io.Writer(os.Stdout)
	logger.AddLogger(stdout, "status")

	db, err := database.NewDBWithDriver(*driver, *dsn)

	if err != nil {
		logger.Fatal("unable to create database (%s) because %s", *dsn, err)
	}

	defer db.Close()

	if *live_hard {

		err = db.LiveHardDieFast()

		if err != nil {
			logger.Fatal("Unable to live hard and die fast so just dying fast instead, because %s", err)
		}
	}

	to_index := make([]sqlite.Table, 0)

	// CHECK FLAGS HERE...

	if *documents || *all {
		docs, err := tables.NewDocumentsTableWithDatabase(db)

		if err != nil {
			logger.Fatal("failed to create documents' table because %s", err)
		}

		to_index = append(to_index, docs)
	}

	if *links || *all {
		docs_links, err := tables.NewDocumentsLinksTableWithDatabase(db)

		if err != nil {
			logger.Fatal("failed to create documents' table because %s", err)
		}

		to_index = append(to_index, docs_links)
	}

	if *authors || *all {
		docs_authors, err := tables.NewDocumentsAuthorsTableWithDatabase(db)

		if err != nil {
			logger.Fatal("failed to create documents' table because %s", err)
		}

		to_index = append(to_index, docs_authors)
	}

	if *search || *all {
		docs_search, err := tables.NewDocumentsSearchTableWithDatabase(db)

		if err != nil {
			logger.Fatal("failed to create documents' table because %s", err)
		}

		to_index = append(to_index, docs_search)
	}

	if len(to_index) == 0 {
		logger.Fatal("You forgot to specify which (any) tables to index")
	}

	cb := func(ctx context.Context, fh io.Reader, args ...interface{}) (interface{}, error) {

		path, err := wof_index.PathForContext(ctx)

		if err != nil {
			return nil, err
		}

		if filepath.Base(path) != *input {
			return nil, nil
		}

		opts := parser.DefaultParseOptions()

		fm, b, err := parser.ParseFile(path, opts)

		if err != nil {
			return nil, err
		}

		doc, err := markdown.NewDocument(fm, b)

		if err != nil {
			logger.Fatal("failed to create new MD document because %s", err)
		}

		return doc, nil
	}

	idx, err := index.NewSQLiteIndexer(db, to_index, cb)

	if err != nil {
		logger.Fatal("failed to create sqlite indexer because %s", err)
	}

	idx.Timings = *timings
	idx.Logger = logger

	err = idx.IndexPaths(*mode, flag.Args())

	if err != nil {
		logger.Fatal("Failed to index paths in %s mode because: %s", *mode, err)
	}

	os.Exit(0)
}
