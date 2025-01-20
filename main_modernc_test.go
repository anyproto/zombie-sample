package test

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/stretchr/testify/require"
	"math/rand"
	"modernc.org/sqlite"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSQLiteOperationsModernc1Conn(t *testing.T) {
	readConn = 1
	runModernc(t)
}

func TestSQLiteOperationsModernc16Conn(t *testing.T) {
	readConn = 16
	runModernc(t)
}

func runModernc(t *testing.T) {
	const initSQL = `
		PRAGMA journal_mode = WAL;
		PRAGMA synchronous = NORMAL;
		PRAGMA temp_store = MEMORY;
		PRAGMA mmap_size = 30000000000; -- 30GB
		PRAGMA busy_timeout = 5000;
		PRAGMA automatic_index = true;
		PRAGMA foreign_keys = ON;
		PRAGMA analysis_limit = 1000;
		PRAGMA trusted_schema = OFF;
	`
	// make sure every opened connection has the settings we expect
	sqlite.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, _ string) error {
		_, err := conn.ExecContext(context.Background(), initSQL, nil)

		return err
	})

	os.Remove("test.db")
	// Open primary database connection
	mainDB, err := sql.Open("sqlite", "file:test.db?cache=shared&mode=rwc&_journal_mode=WAL")
	require.NoError(t, err)
	mainConn, err := mainDB.Conn(context.Background())
	require.NoError(t, err)
	defer mainConn.Close()
	defer mainDB.Close()

	ctx := context.Background()

	// Create collections
	tStart := time.Now()
	tables := make([]string, collN)
	for i := 0; i < collN; i++ {
		tableName := fmt.Sprintf("test_%d", i)
		tx, err := mainConn.BeginTx(ctx, nil)
		require.NoError(t, err)

		createTableSQL := fmt.Sprintf(`CREATE TABLE %s (id INTEGER PRIMARY KEY, data TEXT)`, tableName)

		_, err = tx.Exec(createTableSQL)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)
		tables[i] = tableName
	}
	t.Logf("created %d tables; %v", collN, time.Since(tStart))

	// Prepare read connections
	readDBs := make([]*sql.Conn, readConn)
	for i := 0; i < readConn; i++ {
		readDB, err := sql.Open("sqlite", "file:test.db?cache=shared&mode=rwc&_journal_mode=WAL")
		require.NoError(t, err)
		defer readDB.Close()
		readDBs[i], err = readDB.Conn(ctx)
		defer readDBs[i].Close()
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

			tx, err := mainConn.BeginTx(ctx, nil)
			require.NoError(t, err)

			stmt, err := tx.PrepareContext(ctx, createTableSQL)
			require.NoError(t, err)

			_, err = stmt.ExecContext(ctx)
			require.NoError(t, err)

			err = stmt.Close()
			require.NoError(t, err)

			err = tx.Commit()
			require.NoError(t, err)

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
			_, err := mainDB.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s (id, data) VALUES (?, ?)`, table), i, data)
			require.NoError(t, err)
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
				stmt, err := readDB.PrepareContext(ctx, fmt.Sprintf(`SELECT data FROM %s WHERE id = ?`, table))
				require.NoError(t, err)
				defer stmt.Close()

				id := rand.Intn(docInsertN)

				tx, err := readDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
				if err != nil {
					fmt.Println(err)
					continue
				}
				//require.NoError(t, err)

				rows, err := tx.StmtContext(ctx, stmt).QueryContext(ctx, id)
				require.NoError(t, err)

				if rows.Next() {
					var data string
					err = rows.Scan(&data)
					require.NoError(t, err)
				}
				err = rows.Close()
				require.NoError(t, err)

				err = tx.Commit()
				require.NoError(t, err)

			}
			t.Logf("read connection %d processed %d queries; %v; avg %v/query", connIdx, findIdN, time.Since(tStart), time.Since(tStart)/time.Duration(findIdN/readConn))
		}(i)
	}

	wg.Wait()
	t.Log("Test finished successfully")
}
