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

func createFile(name string) error {
	outFile, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, err = outFile.WriteString("test")
	if err != nil {
		return err
	}
	return outFile.Close()
}

func createItem(db *sql.DB, hash string, expired time.Time) (*Item, error) {
	now := time.Now().UTC()
	item := &Item{
		Name:    "abc",
		Path:    testStorage,
		Salt:    "abc",
		Hash:    hash,
		Counter: 1,
		Created: now,
		Expired: expired,
	}
	err := createFile(item.FullPath())
	if err != nil {
		return nil, err
	}
	err = item.Save(db)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func readIDs(db *sql.DB, t *testing.T) (map[int64]bool, error) {
	var id int64
	ids := make(map[int64]bool)
	stmt, err := db.Prepare("SELECT id FROM `storage` WHERE 1=1;")
	if (err != nil) && (err != sql.ErrNoRows) {
		return nil, err
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err == sql.ErrNoRows {
		return ids, nil // no items
	}
	rows, err := stmt.Query()
	for rows.Next() {
		err = rows.Scan(&id)
		if err != nil {
			return nil, err
		}
		ids[id] = true
	}
	err = rows.Close()
	if err != nil {
		return nil, err
	}
	return ids, nil
}

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
	err = createFile(filepath.Join(testStorage, hash))
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

func TestGCMonitor(t *testing.T) {
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
	// item1 - expired
	_, err = createItem(db, "ab117372d41c05ba9ee4d4ea2f9ebab8e838990e4ff3316bb8c38cfb3ec2afc1", now)
	if err != nil {
		t.Fatal(err)
	}
	// item2 - not expired, but deleted by id
	afterHour := now.Add(time.Hour)
	item2, err := createItem(db, "ab117372d41c05ba9ee4d4ea2f9ebab8e838990e4ff3316bb8c38cfb3ec2afc2", afterHour)
	if err != nil {
		t.Fatal(err)
	}
	// item3 - not expired, not deleted
	item3, err := createItem(db, "ab117372d41c05ba9ee4d4ea2f9ebab8e838990e4ff3316bb8c38cfb3ec2afc3", afterHour)
	if err != nil {
		t.Fatal(err)
	}

	ids, err := readIDs(db, t)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(ids); n != 3 {
		t.Errorf("failed len=%v", n)
	}
	closing := make(chan struct{})
	monitoring := make(chan *Item)
	period := 200 * time.Millisecond

	go GCMonitor(monitoring, closing, db, loggerInfo, loggerInfo, period)

	time.Sleep(period * 2) // delete item1
	monitoring <- item2    // delete item2
	time.Sleep(period * 2) // wait item2 deleting

	ids, err = readIDs(db, t)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(ids); n != 1 {
		t.Errorf("failed len=%v: %v", n, ids)
	}
	if !ids[item3.ID] {
		t.Error("no found item")
	}
	close(closing)
	time.Sleep(period)
	close(monitoring)

	err = item3.Delete(db, loggerInfo)
	if err != nil {
		t.Error(err)
	}
}

func TestItem_IsFileExists(t *testing.T) {
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
	item, err := createItem(db, "ab117372d41c05ba9ee4d4ea2f9ebab8e838990e4ff3316bb8c38cfb3ec2afc4", now)
	if err != nil {
		t.Fatal(err)
	}
	if !item.IsFileExists() {
		t.Error("file does not exist")
	}
	err = item.Delete(db, loggerInfo)
	if err != nil {
		t.Error(err)
	}
	if item.IsFileExists() {
		t.Error("file exists")
	}
}

func TestItem_Decrement(t *testing.T) {
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
	item, err := createItem(db, "ab117372d41c05ba9ee4d4ea2f9ebab8e838990e4ff3316bb8c38cfb3ec2afc5", now)
	if err != nil {
		t.Fatal(err)
	}
	if item.Counter != 1 {
		t.Error("failed item counter")
	}
	ok, err := item.Decrement(db)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("no decrement update")
	}
	if item.Counter != 0 {
		t.Error("failed item counter")
	}
	err = item.Delete(db, loggerInfo)
	if err != nil {
		t.Error(err)
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
