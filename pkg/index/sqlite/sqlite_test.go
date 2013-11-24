/*
Copyright 2012 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sqlite_test

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sync"
	"testing"

	"camlistore.org/pkg/index"
	"camlistore.org/pkg/index/indextest"
	"camlistore.org/pkg/index/sqlite"
	"camlistore.org/pkg/sorted"

	_ "camlistore.org/third_party/github.com/mattn/go-sqlite3"
)

var (
	once        sync.Once
	dbAvailable bool
	rootdb      *sql.DB
)

func do(db *sql.DB, sql string) {
	_, err := db.Exec(sql)
	if err == nil {
		return
	}
	panic(fmt.Sprintf("Error %v running SQL: %s", err, sql))
}

func makeStorage(t *testing.T) (s sorted.KeyValue, clean func()) {
	f, err := ioutil.TempFile("", "sqlite-test")
	if err != nil {
		t.Fatal(err)
	}
	clean = func() {
		os.Remove(f.Name())
	}
	db, err := sql.Open("sqlite3", f.Name())
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	for _, tableSql := range sqlite.SQLCreateTables() {
		do(db, tableSql)
	}
	do(db, fmt.Sprintf(`REPLACE INTO meta VALUES ('version', '%d')`, sqlite.SchemaVersion()))
	s, err = sqlite.NewStorage(f.Name())
	if err != nil {
		panic(err)
	}
	return s, clean
}

type sqliteTester struct{}

func (sqliteTester) test(t *testing.T, tfn func(*testing.T, func() *index.Index)) {
	var mu sync.Mutex // guards cleanups
	var cleanups []func()
	defer func() {
		mu.Lock() // never unlocked
		for _, fn := range cleanups {
			fn()
		}
	}()
	makeIndex := func() *index.Index {
		s, cleanup := makeStorage(t)
		mu.Lock()
		cleanups = append(cleanups, cleanup)
		mu.Unlock()
		return index.New(s)
	}
	tfn(t, makeIndex)
}

func TestIndex_SQLite(t *testing.T) {
	sqliteTester{}.test(t, indextest.Index)
}

func TestPathsOfSignerTarget_SQLite(t *testing.T) {
	sqliteTester{}.test(t, indextest.PathsOfSignerTarget)
}

func TestFiles_SQLite(t *testing.T) {
	sqliteTester{}.test(t, indextest.Files)
}

func TestEdgesTo_SQLite(t *testing.T) {
	sqliteTester{}.test(t, indextest.EdgesTo)
}

func TestIsDeleted_SQLite(t *testing.T) {
	sqliteTester{}.test(t, indextest.IsDeleted)
}

func TestDeletedAt_SQLite(t *testing.T) {
	sqliteTester{}.test(t, indextest.DeletedAt)
}

func TestConcurrency(t *testing.T) {
	if testing.Short() {
		t.Logf("skipping for short mode")
		return
	}
	s, clean := makeStorage(t)
	defer clean()
	const n = 100
	ch := make(chan error)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			bm := s.BeginBatch()
			bm.Set("keyA-"+fmt.Sprint(i), fmt.Sprintf("valA=%d", i))
			bm.Set("keyB-"+fmt.Sprint(i), fmt.Sprintf("valB=%d", i))
			ch <- s.CommitBatch(bm)
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-ch; err != nil {
			t.Errorf("%d: %v", i, err)
		}
	}
}

func numFDs(t *testing.T) int {
	lsofPath, err := exec.LookPath("lsof")
	if err != nil {
		t.Skipf("No lsof available; skipping test")
	}
	out, err := exec.Command(lsofPath, "-n", "-p", fmt.Sprint(os.Getpid())).Output()
	if err != nil {
		t.Skipf("Error running lsof; skipping test: %s", err)
	}
	return bytes.Count(out, []byte("\n")) - 1 // hacky
}

func TestFDLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode.")
	}
	fd0 := numFDs(t)
	t.Logf("fd0 = %d", fd0)

	s, clean := makeStorage(t)
	defer clean()

	bm := s.BeginBatch()
	const numRows = 150 // 3x the batchSize of 50 in sqlindex.go; to gaurantee we do multiple batches
	for i := 0; i < numRows; i++ {
		bm.Set(fmt.Sprintf("key:%05d", i), fmt.Sprint(i))
	}
	if err := s.CommitBatch(bm); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		it := s.Find("key:")
		n := 0
		for it.Next() {
			n++
		}
		if n != numRows {
			t.Errorf("iterated over %d rows; want %d", n, numRows)
		}
		it.Close()
		t.Logf("fd after iteration %d = %d", i, numFDs(t))
	}
}
