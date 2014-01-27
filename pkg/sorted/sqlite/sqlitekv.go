/*
Copyright 2012 The Camlistore Authors.

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

// Package sqlite provides an implementation of sorted.KeyValue
// using an SQLite database file.
package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	"camlistore.org/pkg/jsonconfig"
	"camlistore.org/pkg/sorted"
	"camlistore.org/pkg/sorted/sqlkv"
)

func init() {
	sorted.RegisterKeyValue("sqlite", newKeyValueFromConfig)
}

func newKeyValueFromConfig(cfg jsonconfig.Obj) (sorted.KeyValue, error) {
	file := cfg.RequiredString("file")
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return NewKeyValue(file)
}

// NewKeyValue returns a KeyValue implementation on top of
// an SQLite database file.
func NewKeyValue(file string) (sorted.KeyValue, error) {
	if !compiled {
		return nil, ErrNotCompiled
	}

	fi, err := os.Stat(file)
	if os.IsNotExist(err) || (err == nil && fi.Size() == 0) {
		return nil, fmt.Errorf(`You need to initialize your SQLite database with: camtool dbinit --dbname=%s --dbtype=sqlite`, file)
	}
	db, err := sql.Open("sqlite3", file)
	if err != nil {
		return nil, err
	}
	kv := &keyValue{
		file: file,
		db:   db,
		KeyValue: &sqlkv.KeyValue{
			DB:     db,
			Serial: true,
		},
	}

	version, err := kv.SchemaVersion()
	if err != nil {
		return nil, fmt.Errorf("error getting schema version (need to init database with 'camtool dbinit %s'?): %v", file, err)
	}

	if err := kv.ping(); err != nil {
		return nil, err
	}

	if version != requiredSchemaVersion {
		if os.Getenv("CAMLI_DEV_CAMLI_ROOT") != "" {
			// Good signal that we're using the devcam server, so help out
			// the user with a more useful tip:
			return nil, fmt.Errorf("database schema version is %d; expect %d (run \"devcam server --wipe\" to wipe both your blobs and re-populate the database schema)", version, requiredSchemaVersion)
		}
		return nil, fmt.Errorf("database schema version is %d; expect %d (need to re-init/upgrade database?)",
			version, requiredSchemaVersion)
	}

	return kv, nil

}

type keyValue struct {
	*sqlkv.KeyValue

	file string
	db   *sql.DB
}

var compiled = false

// CompiledIn returns whether SQLite support is compiled in.
// If it returns false, the build tag "with_sqlite" was not specified.
func CompiledIn() bool {
	return compiled
}

var ErrNotCompiled = errors.New("camlistored was not built with SQLite support. If you built with make.go, use go run make.go --sqlite=true. If you used go get or get install, use go {get,install} --tags=with_sqlite" + compileHint())

func compileHint() string {
	if _, err := os.Stat("/etc/apt"); err == nil {
		return " (Hint: apt-get install libsqlite3-dev)"
	}
	return ""
}

func (kv *keyValue) ping() error {
	// TODO(bradfitz): something more efficient here?
	_, err := kv.SchemaVersion()
	return err
}

func (kv *keyValue) SchemaVersion() (version int, err error) {
	err = kv.db.QueryRow("SELECT value FROM meta WHERE metakey='version'").Scan(&version)
	return
}
