package test

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"math/rand"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSQLiteOperationsMattn1Conn(t *testing.T) {
	readConn = 1
	runMattn(t)
}

func TestSQLiteOperationsMattn16Conn(t *testing.T) {
	readConn = 16
	runMattn(t)
}

func runMattn(t *testing.T) {
	os.Remove("test.db")
	// Open primary database connection
	db, err := sql.Open("sqlite3", "file:test.db?cache=shared&mode=rwc&_journal_mode=WAL")
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(readConn + 1)

	ctx := context.Background()

	// Create tables
	tStart := time.Now()
	tables := make([]string, collN)
	for i := 0; i < collN; i++ {
		tableName := fmt.Sprintf("test_%d", i)
		createTableSQL := fmt.Sprintf(`CREATE TABLE %s (id INTEGER PRIMARY KEY, data TEXT)`, tableName)
		_, err = db.Exec(createTableSQL)
		require.NoError(t, err)
		tables[i] = tableName
	}
	t.Logf("created %d tables; %v", collN, time.Since(tStart))

	// Prepare read connections
	readDBs := make([]*sql.Conn, readConn)
	for i := 0; i < readConn; i++ {
		readDBs[i], err = db.Conn(ctx)
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

			_, _ = db.Exec(createTableSQL)
			//require.NoError(t, err)

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
			_, _ = db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s (id, data) VALUES (?, ?)`, table), i, data)
			//require.NoError(t, err)
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

				id := rand.Intn(docInsertN)
				rows, err := readDB.QueryContext(ctx, fmt.Sprintf(`SELECT data FROM %s WHERE id = ?`, table), id)
				require.NoError(t, err)

				if rows.Next() {
					var data string
					err = rows.Scan(&data)
					require.NoError(t, err)
				}
				err = rows.Close()
				require.NoError(t, err)
			}
			t.Logf("read connection %d processed %d queries; %v; avg %v/query", connIdx, findIdN, time.Since(tStart), time.Since(tStart)/time.Duration(findIdN/readConn))
		}(i)
	}

	wg.Wait()
	t.Log("Test finished successfully")
}
