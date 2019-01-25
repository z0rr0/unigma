package db

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/sha3"
)

const (
	// saltSize is random salt, also used for storage file name
	saltSize = 128
	// pbkdf2Iter is number of pbkdf2 iterations
	pbkdf2Iter = 16384
	// key length for AES-256
	aesKeyLength = 32
	// hashLength is length of file hash.
	hashLength = 32
)

// Item is base data struct for incoming data.
type Item struct {
	ID      int
	Name    string
	Path    string
	Salt    string
	Hash    string
	Counter int
	Created time.Time
	Expired time.Time
}

// ContentType returns string content-type for stored file.
func (item *Item) ContentType() string {
	var ext string
	i := strings.LastIndex(item.Name, ".")
	if i > -1 {
		ext = item.Name[i:]
	}
	m := mime.TypeByExtension(ext)
	if m == "" {
		return "application/octet-stream"
	}
	return m
}

// FullPath return full path for item's file.
func (item *Item) FullPath() string {
	return filepath.Join(item.Path, item.Hash)
}

// Key calculates and returns secret key and its SHA512 hash.
func Key(secret string, salt []byte) ([]byte, []byte) {
	key := pbkdf2.Key([]byte(secret), salt, pbkdf2Iter, aesKeyLength, sha3.New512)
	b := make([]byte, hashLength)
	sha3.ShakeSum256(b, append(key, salt...))
	return key, b
}

// IsValidSecret checks the secret.
func (item *Item) IsValidSecret(secret string) ([]byte, error) {
	salt, err := hex.DecodeString(item.Salt)
	if err != nil {
		return nil, err
	}
	hash, err := hex.DecodeString(item.Hash)
	if err != nil {
		return nil, err
	}
	key, keyHash := Key(secret, salt)
	if !hmac.Equal(hash, keyHash) {
		return nil, errors.New("failed password")
	}
	return key, nil
}

// Encrypt encrypts source file and fills the item by result.
func (item *Item) Encrypt(inFile io.Reader, secret string, l *log.Logger) error {
	salt := make([]byte, saltSize)
	_, err := rand.Read(salt)
	if err != nil {
		return err
	}
	key, keyHash := Key(secret, salt)
	item.Hash = hex.EncodeToString(keyHash)
	// check file exists
	fullPath := item.FullPath()
	if item.IsFileExists() {
		return fmt.Errorf("file %v already exists", fullPath)
	}
	item.Salt = hex.EncodeToString(salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	// the key is unique for each cipher-text, then it's ok to use a zero IV.
	var iv [aes.BlockSize]byte
	stream := cipher.NewOFB(block, iv[:])
	outFile, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if err := outFile.Close(); err != nil {
			l.Printf("close encypted file error: %v", err)
		}
	}()
	writer := &cipher.StreamWriter{S: stream, W: outFile}
	// copy the input file to the output file, encrypting as we go.
	if _, err := io.Copy(writer, inFile); err != nil {
		return err
	}
	return nil
}

// Decrypt decrypts item related file and writes result to w.
func (item *Item) Decrypt(w io.Writer, key []byte, l *log.Logger) error {
	fileName := filepath.Join(item.Path, item.Hash)
	inFile, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer func() {
		if err := inFile.Close(); err != nil {
			l.Printf("close in-encypted file error: %v", err)
		}
	}()
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	// if the key is unique for each cipher-text, then it's ok to use a zero IV.
	var iv [aes.BlockSize]byte
	stream := cipher.NewOFB(block, iv[:])

	reader := &cipher.StreamReader{S: stream, R: inFile}
	// copy the input file to the output file, decrypting as we go.
	if _, err := io.Copy(w, reader); err != nil {
		return err
	}
	return nil
}

// GetURL returns item's URL.
func (item *Item) GetURL(r *http.Request, secure bool) *url.URL {
	// r.URL.Scheme is blank, so use hint from settings
	scheme := "http"
	if secure {
		scheme = "https"
	}
	return &url.URL{
		Scheme: scheme,
		Host:   r.Host,
		Path:   item.Hash,
	}
}

// IsFileExists checks item's related file exists.
func (item *Item) IsFileExists() bool {
	_, err := os.Stat(item.FullPath())
	if err == nil {
		return true
	}
	return false
}

// Save saves the item to database.
func (item *Item) Save(db *sql.DB) error {
	stmt, err := db.Prepare("INSERT INTO `storage` (`name`, `path`, `hash`, `salt`, `counter`, `created`, `updated`, `expired`) values (?, ?, ?, ?, ?, ?, ?, ?);")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(item.Name, item.Path, item.Hash, item.Salt, item.Counter, item.Created, item.Created, item.Expired)
	if err != nil {
		return err
	}
	return stmt.Close()
}

// Decrement updates items' counter.
func (item *Item) Decrement(db *sql.DB) (bool, error) {
	stmt, err := db.Prepare("UPDATE `storage` SET `counter`=`counter`-1, `updated`=? WHERE `counter`>0 AND `id`=?;")
	if err != nil {
		return false, err
	}
	_, err = stmt.Exec(time.Now().UTC(), item.ID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	err = stmt.Close()
	if err != nil {
		return false, err
	}
	item.Counter--
	return true, nil
}

// Delete removes items from database and related file from file system.
func (item *Item) Delete(db *sql.DB, le *log.Logger) error {
	stmt, err := db.Prepare("DELETE FROM `storage` WHERE `id`=?;")
	if err != nil {
		return fmt.Errorf("failed prepare item delete by id: %v", err)
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			le.Printf("failed close stmt: %v\n", err)
		}
	}()
	_, err = stmt.Exec(item.ID)
	if err != nil {
		return fmt.Errorf("failed item delete by id: %v", err)
	}
	return os.Remove(item.FullPath())
}

// Read reads an item by its hash from database.
func Read(db *sql.DB, hash string) (*Item, error) {
	stmt, err := db.Prepare("SELECT `id`, `name`, `path`, `hash`, `salt`, `counter`, `created`, `expired` FROM `storage` WHERE `counter`>0 AND `hash`=?;")
	if err != nil {
		return nil, err
	}
	item := &Item{}
	err = stmt.QueryRow(hash).Scan(
		&item.ID,
		&item.Name,
		&item.Path,
		&item.Hash,
		&item.Salt,
		&item.Counter,
		&item.Created,
		&item.Expired,
	)
	if err == sql.ErrNoRows {
		return item, nil
	}
	if err != nil {
		return nil, err
	}
	err = stmt.Close()
	if err != nil {
		return nil, err
	}
	return item, nil
}

func deleteByDate(db *sql.DB, le *log.Logger) (int, error) {
	var (
		paths []string
		ids   []string
	)
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare("SELECT `id`, `path`, `hash` FROM `storage` WHERE `expired`<?;")
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, tx.Commit()
		}
		return 0, fmt.Errorf("%v, commit err=%v", err, tx.Rollback())
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			le.Printf("failed close stmt: %v\n", err)
		}
	}()
	rows, err := stmt.Query(time.Now().UTC())
	item := &Item{} // use only one item to collect paths
	for rows.Next() {
		err = rows.Scan(&item.ID, &item.Path, &item.Hash)
		if err != nil {
			return 0, fmt.Errorf("%v, commit err=%v", err, tx.Rollback())
		}
		paths = append(paths, item.FullPath())
		ids = append(ids, strconv.Itoa(item.ID))
	}
	err = rows.Close()
	if err != nil {
		return 0, fmt.Errorf("%v, commit err=%v", err, tx.Rollback())
	}
	// delete items from db
	stmt, err = tx.Prepare("DELETE FROM `storage` WHERE `id` IN (?);")
	if err != nil {
		return 0, fmt.Errorf("%v, commit err=%v", err, tx.Rollback())
	}
	_, err = stmt.Exec(strings.Join(ids, ","))
	if err != nil {
		return 0, fmt.Errorf("%v, commit err=%v", err, tx.Rollback())
	}
	err = tx.Commit()
	if err != nil {
		return 0, err
	}
	for _, p := range paths {
		err = os.RemoveAll(p)
		if err != nil {
			return 0, err
		}
	}
	return len(paths), nil
}

// GCMonitor is garbage collection monitoring to delete expired by date or counter items.
func GCMonitor(ch <-chan *Item, db *sql.DB, li, le *log.Logger, period time.Duration) {
	tc := time.Tick(period)
	li.Printf("GC monitor is running, perid=%v\n", period)
	for {
		select {
		case item := <-ch:
			if err := item.Delete(db, le); err != nil {
				le.Println(err)
			} else {
				li.Printf("deleted item=%v\n", item.ID)
			}
		case <-tc:
			if n, err := deleteByDate(db, le); err != nil {
				le.Println(err)
			} else {
				if n > 0 {
					li.Printf("deleted %v expired items\n", n)
				}
			}
		}
	}
}
