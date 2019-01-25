package db

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite3 driver package
)

const (
	testDB = "/tmp/db.sqlite"
)

func TestRead(t *testing.T) {
	db, err := sql.Open("sqlite3", testDB)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	now := time.Now().UTC()
	stmt, err := db.Prepare("INSERT INTO `storage` (`name`, `path`, `hash`, `salt`, `created`, `updated`, `expired`) values (?, ?, ?, ?, ?, ?, ?);")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			t.Error(err)
		}
	}()
	hash := "12345"
	_, err = stmt.Exec("", "", hash, "abc", now, now, now)
	if err != nil {
		t.Fatal(err)
	}
	err = stmt.Close()
	if err != nil {
		t.Fatal(err)
	}
	item, err := Read(db, hash)
	if err != nil {
		t.Fatal(err)
	}
	if (item.Counter != 1) || (item.ID < 1) {
		t.Error("failed read")
	}
	return
}

func BenchmarkKey(b *testing.B) {
	secret, salt := "secret", []byte("abcdefgabcdefgabcdefgabcdefgabcdefgabcdefgabcdefgabcdefgabcdefga")
	for n := 0; n < b.N; n++ {
		key, h := Key(secret, salt)
		if (len(key) == 0) || (len(h) == 0) {
			b.Error("unexpected error")
		}
	}
}
