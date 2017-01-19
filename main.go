package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	// sqlite driver
	_ "github.com/mattn/go-sqlite3"

	// magic functionality
	"github.com/rakyll/magicmime"
)

const (
	// each progressStep notification is thrown
	progressStep = 100

	capacity = 1024
	workers  = 8
)

type request struct {
	path string
	f    os.FileInfo
}

type entry struct {
	path      string
	basename  string
	directory bool
	tipo      string
	modified  time.Time
	size      int64
}

var (
	conn           *sql.DB
	filesProcessed = 0
	lastChunk      = time.Now()
	requests       = make(chan *request, capacity)
	wg             = sync.WaitGroup{}
	write          = sync.Mutex{}
)

func visit(path string, f os.FileInfo, err error) error {

	// skip non-reachable dir
	if err != nil {
		log.Println("Warning, skipping because of error:", err)
		return nil
	}

	wg.Add(1)
	requests <- &request{
		path: path,
		f:    f,
	}

	return nil
}

func worker() {

	var entries []entry

	for {

		if len(entries) > capacity { // flush if too many requests
			flush(entries)
			entries = nil
		}

		select {
		case <-time.Tick(time.Second): // flush on idle
			flush(entries)
			entries = nil
		case msg := <-requests:

			var fileOutput = ""

			// only for files get magic
			if !msg.f.IsDir() && !strings.Contains(msg.f.Name(), ".") {
				magic, err := magicmime.TypeByFile(msg.path)
				if err == nil {
					fileOutput = magic
				}
			}

			entries = append(entries, entry{
				msg.path,
				msg.f.Name(),
				msg.f.IsDir(),
				fileOutput,
				msg.f.ModTime(),
				msg.f.Size(),
			})

		}
	}

}

func flush(entries []entry) {
	write.Lock()

	// begin TX -- more effective way to write tons of data
	tx, err := conn.Begin()
	if err != nil {
		log.Fatal(err)
	}

	// create STMT for fast insert
	stmt, err := tx.Prepare(`INSERT INTO files(
				path, basename, directory, type, modified, size
			) VALUES (
				?,    ?,        ?,         ?,    ?,        ?
			)`)
	if err != nil {
		log.Println(err)
		return
	}
	defer stmt.Close()

	// write queries
	for _, e := range entries {
		stmt.Exec(e.path, e.basename, e.directory, e.tipo, e.modified, e.size)
	}

	// finish TX
	tx.Commit()

	for range entries {
		wg.Done()
		filesProcessed++
	}

	log.Printf("Processing speed: %v fps", float64(filesProcessed)/time.Since(lastChunk).Seconds())
	lastChunk = time.Now()

	write.Unlock()
}

func main() {

	// check args
	if len(os.Args) < 3 {
		log.Fatal(fmt.Errorf("Usage: %v directory sqlitedb", os.Args[0]))
	}

	// init magic lib
	if err := magicmime.Open(magicmime.MAGIC_MIME_TYPE | magicmime.MAGIC_SYMLINK | magicmime.MAGIC_ERROR); err != nil {
		log.Fatal(err)
	}
	defer magicmime.Close()

	// open database
	db, err := sql.Open("sqlite3", os.Args[2])
	if err != nil {
		log.Fatal(err)
	}
	conn = db
	defer db.Close()

	// set mode to WAL
	_, err = db.Exec(`PRAGMA journal_mode = WAL`)
	if err != nil {
		log.Println(err) // no more fatal as we need defer
		return
	}

	// make sure table exists
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS
	files (
		path text not null primary key,
		basename text,
		directory bool,
		type text,
		modified datetime,
		size integer
	) `)
	if err != nil {
		log.Println(err)
		return
	}

	// start workers
	for i := 1; i <= workers; i++ {
		go worker()
	}

	err = filepath.Walk(os.Args[1], visit)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("Finished; waiting for last files to upd")
	wg.Wait()

}
