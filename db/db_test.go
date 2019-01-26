package db

import (
	"bytes"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite3 driver package
)

const (
	testDB      = "/tmp/unigma_db.sqlite"
	testStorage = "/tmp/unigma_storage"
)

var (
	loggerInfo = log.New(os.Stdout, "[TEST]", log.Ltime|log.Lshortfile)
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
	hash := "12345"
	outFile, err := os.OpenFile(filepath.Join(testStorage, hash), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = outFile.WriteString("test")
	if err != nil {
		t.Fatal(err)
	}
	err = outFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	_, err = stmt.Exec("", testStorage, hash, "abc", now, now, now)
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
	err = item.Delete(db, loggerInfo)
	if err != nil {
		t.Errorf("failed delete: %v", err)
	}
	err = stmt.Close()
	if err != nil {
		t.Fatal(err)
	}
	return
}

func TestKey(t *testing.T) {
	secret, salt := "secret", []byte("abcdefgabcdefgabcdefgabcdefgabcdefgabcdefgabcdefgabcdefgabcdefga")
	key1, h1 := Key(secret, salt)
	key2, h2 := Key(secret, salt)
	if n := bytes.Compare(key1, key2); n != 0 {
		t.Errorf("Failed compare keys: %v", n)
	}
	if n := bytes.Compare(h1, h2); n != 0 {
		t.Errorf("Failed compare keys: %v", n)
	}
}

func TestIsNameHash(t *testing.T) {
	values := map[string]bool{
		"":  false,
		"a": false,
		"ab117372d41c05ba9ee4d4ea2f9ebab8e838990e4ff3316bb8c38cfb3ec2afc6":  true,
		"ab117372d41c05ba9ee4d4ea2f9ebab8e838990e4ff3316bb8c38cfb3ec2afc8":  true,
		"ab117372d41c05ba9ee4d4ea2f9ebab8e838990e4ff3316bb8c38cfb3ec2afcz":  false,
		"ab117372d41c05ba9ee4d4ea2f9ebab8e838990e4ff3316bb8c38cfb3ec2afc8a": false,
	}
	for h, r := range values {
		v := IsNameHash(h)
		if r != v {
			t.Errorf("failed hash name: %v", h)
		}
	}
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
