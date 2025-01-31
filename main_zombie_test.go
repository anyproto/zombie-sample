package test

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	_ "net/http/pprof"
)

var (
	collN      = 100
	docInsertN = 300000
	findIdN    = 10000
	readConn   = 0
	extraCollN = 1000
)

func TestSQLiteOperationsZombie1Conn(t *testing.T) {
	readConn = 1
	runZombie(t)
}

func TestSQLiteOperationsZombie16Conn(t *testing.T) {
	readConn = 16
	runZombie(t)
}

func runZombie(t *testing.T) {
	dbPath, _ := os.MkdirTemp("", "sqlite-*")
	dbPath = filepath.Join(dbPath, "test.db")
	print(dbPath)

	// Open primary database connection
	mainDB, err := sqlite.OpenConn(dbPath, sqlite.OpenCreate|sqlite.OpenWAL|sqlite.OpenURI|sqlite.OpenReadWrite)
	require.NoError(t, err)
	defer mainDB.Close()
	mainDB2, err := sqlite.OpenConn(dbPath, sqlite.OpenCreate|sqlite.OpenWAL|sqlite.OpenURI|sqlite.OpenReadWrite)
	require.NoError(t, err)
	defer mainDB2.Close()

	// Create collections
	tStart := time.Now()
	tables := make([]string, collN)
	for i := 0; i < collN; i++ {
		tableName := fmt.Sprintf("test_%d", i)

		err := sqlitex.ExecuteTransient(mainDB, "BEGIN TRANSACTION;", nil)
		if err != nil {
			log.Fatalf("failed to begin transaction: %v", err)
		}

		createTableSQL := fmt.Sprintf(`CREATE TABLE %s (id INTEGER PRIMARY KEY, data TEXT)`, tableName)

		err = sqlitex.ExecuteTransient(mainDB, createTableSQL, nil)
		if err != nil {
			log.Printf("failed to create table %s: %v", tableName, err)
			rollbackErr := sqlitex.ExecuteTransient(mainDB, "ROLLBACK;", nil)
			if rollbackErr != nil {
				log.Fatalf("failed to rollback transaction: %v", rollbackErr)
			}
			continue
		}

		err = sqlitex.ExecuteTransient(mainDB, "COMMIT;", nil)
		if err != nil {
			log.Fatalf("failed to commit transaction: %v", err)
		}

		tables[i] = tableName
	}
	t.Logf("created %d tables; %v", collN, time.Since(tStart))

	// Prepare read connections
	readDBs := make([]*sqlite.Conn, readConn)
	for i := 0; i < readConn; i++ {
		readDB, err := sqlite.OpenConn(dbPath, sqlite.OpenWAL|sqlite.OpenURI|sqlite.OpenReadOnly)
		require.NoError(t, err)
		readDBs[i] = readDB
		defer readDB.Close()
		require.NoError(t, err)
	}
	t.Logf("prepared %d read connections", readConn)

	var wg sync.WaitGroup

	// Extra collections creation
	wg.Add(1)
	go func() {
		defer wg.Done()
		tStart := time.Now()
		for i := 0; i < extraCollN; i++ {
			tableName := fmt.Sprintf("test_extra_%d", i)
			createTableSQL := fmt.Sprintf(`CREATE TABLE %s (id INTEGER PRIMARY KEY, data TEXT)`, tableName)

			stmt, err := mainDB2.Prepare(createTableSQL)
			if err != nil {
				log.Printf("failed to prepare statement for %s: %v", tableName, err)
				continue
			}

			_, err = stmt.Step()
			if err != nil {
				log.Printf("failed to execute statement for %s: %v", tableName, err)
				stmt.Finalize()
				continue
			}

			err = stmt.Finalize()
			if err != nil {
				log.Fatalf("failed to finalize statement: %v", err)
			}

			time.Sleep(10 * time.Millisecond)
		}
		t.Logf("created %d extra tables; %v", extraCollN, time.Since(tStart))
	}()

	// Insert data
	wg.Add(1)
	go func() {
		defer wg.Done()
		tStart := time.Now()
		data := strings.Repeat("X", 1024)

		for i := 0; i < docInsertN; i++ {
			table := tables[rand.Intn(len(tables))]
			insertSQL := fmt.Sprintf(`INSERT INTO %s (id, data) VALUES (?, ?)`, table)

			stmt, err := mainDB.Prepare(insertSQL)
			if err != nil {
				log.Printf("failed to prepare statement for %s: %v", table, err)
				continue
			}

			stmt.BindInt64(1, int64(i))

			stmt.BindText(2, data)
			_, err = stmt.Step()
			if err != nil {
				log.Printf("failed to execute statement for %s: %v", table, err)
				stmt.Finalize()
				_ = sqlitex.ExecuteTransient(mainDB, "ROLLBACK;", nil)
				continue
			}

			err = stmt.Finalize()
			if err != nil {
				log.Fatalf("failed to finalize statement: %v", err)
			}

		}
		t.Logf("inserted %d rows; %v", docInsertN, time.Since(tStart))
	}()

	// Read data
	for i := 0; i < readConn; i++ {
		wg.Add(1)
		go func(connIdx int) {
			defer wg.Done()
			tStart := time.Now()
			readDB := readDBs[connIdx]

			for j := 0; j < findIdN; j++ {
				table := tables[rand.Intn(len(tables))]
				selectSQL := fmt.Sprintf(`SELECT data FROM %s WHERE id = ?`, table)

				err := sqlitex.ExecuteTransient(readDB, "BEGIN TRANSACTION;", nil)
				if err != nil {
					log.Printf("failed to begin transaction: %v", err)
					continue
				}

				stmt, err := readDB.Prepare(selectSQL)
				if err != nil {
					log.Printf("failed to prepare statement: %v", err)
					_ = sqlitex.ExecuteTransient(readDB, "ROLLBACK;", nil)
					continue
				}

				id := rand.Intn(docInsertN)
				stmt.BindInt64(1, int64(id))

				hasRow, err := stmt.Step()
				if err != nil {
					log.Printf("failed to execute query: %v", err)
					stmt.Finalize()
					_ = sqlitex.ExecuteTransient(readDB, "ROLLBACK;", nil)
					continue
				}

				if hasRow {
					_ = stmt.ColumnText(0)
					//fmt.Printf("Found data: %s\n", data)
				}

				err = stmt.Finalize()
				if err != nil {
					log.Fatalf("failed to finalize statement: %v", err)
				}

				err = sqlitex.ExecuteTransient(readDB, "COMMIT;", nil)
				if err != nil {
					log.Fatalf("failed to commit transaction: %v", err)
				}
			}
			t.Logf("read connection %d processed %d queries; %v; avg %v/query", connIdx, findIdN, time.Since(tStart), time.Since(tStart)/time.Duration(findIdN/readConn))
		}(i)
	}

	wg.Wait()
	t.Log("Test finished successfully")
}
