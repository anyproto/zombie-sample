package test

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
	"log"
	"math/rand"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSQLiteOperationsBBoltConn(t *testing.T) {
	readConn = 1
	runBbolt(t)
}

func TestSQLiteOperationsBbolt16Conn(t *testing.T) {
	readConn = 16
	runBbolt(t)
}

func runBbolt(t *testing.T) {
	dbPath, _ := os.MkdirTemp("", "bbolt-*")
	dbPath = filepath.Join(dbPath, "test.db")
	print(dbPath)

	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	require.NoError(t, err)
	defer db.Close()

	var wg sync.WaitGroup

	tStart := time.Now()
	err = db.Update(func(tx *bbolt.Tx) error {
		for i := 0; i < collN; i++ {
			_, err := tx.CreateBucketIfNotExists([]byte(fmt.Sprintf("test_%d", i)))
			if err != nil {
				return fmt.Errorf("failed to create bucket: %v", err)
			}
		}
		return nil
	})
	require.NoError(t, err)
	t.Logf("created %d buckets; %v", collN, time.Since(tStart))

	wg.Add(1)
	go func() {
		defer wg.Done()
		tStart := time.Now()
		err := db.Update(func(tx *bbolt.Tx) error {
			for i := 0; i < extraCollN; i++ {
				_, err := tx.CreateBucketIfNotExists([]byte(fmt.Sprintf("test_extra_%d", i)))
				if err != nil {
					log.Printf("failed to create extra bucket: %v", err)
					continue
				}
				time.Sleep(10 * time.Millisecond)
			}
			return nil
		})
		require.NoError(t, err)
		t.Logf("created %d extra buckets; %v", extraCollN, time.Since(tStart))
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		tStart := time.Now()
		data := []byte(strings.Repeat("X", 1024))

		err := db.Batch(func(tx *bbolt.Tx) error {
			for i := 0; i < docInsertN; i++ {
				bucket := tx.Bucket([]byte(fmt.Sprintf("test_%d", rand.Intn(collN))))
				if bucket == nil {
					return fmt.Errorf("bucket not found")
				}
				key := []byte(fmt.Sprintf("%d", i))
				bucket.Put(key, data)
			}
			return nil
		})
		require.NoError(t, err)
		t.Logf("inserted %d records; %v", docInsertN, time.Since(tStart))
	}()

	for i := 0; i < readConn; i++ {
		wg.Add(1)
		go func(connIdx int) {
			defer wg.Done()
			tStart := time.Now()

			err := db.View(func(tx *bbolt.Tx) error {
				for j := 0; j < findIdN; j++ {
					bucket := tx.Bucket([]byte(fmt.Sprintf("test_%d", rand.Intn(collN))))
					if bucket == nil {
						return fmt.Errorf("bucket not found")
					}
					key := []byte(fmt.Sprintf("%d", rand.Intn(docInsertN)))
					x := bucket.Get(key)
					if x != nil {
						print(x)
					}
				}
				return nil
			})
			require.NoError(t, err)
			t.Logf("read connection %d processed %d queries; %v; avg %v/query", connIdx, findIdN, time.Since(tStart), time.Since(tStart)/time.Duration(findIdN/readConn))
		}(i)
	}

	//db.View(func(tx *bbolt.Tx) error {
	//	f, _ := os.Create("new.db")
	//	to, err := tx.WriteTo(f)
	//	print(to)
	//	if err != nil {
	//		return err
	//	}
	//	return nil
	//})

	wg.Wait()
	t.Log("Test finished successfully")
}
