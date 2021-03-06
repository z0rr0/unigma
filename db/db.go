// Copyright 2020 Alexander Zaytsev <me@axv.email>.
// All rights reserved. Use of this source code is governed
// by a MIT-style license that can be found in the LICENSE file.

// Package db contains methods to store files on filesystem and metadata in database.
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
	"regexp"
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
	pbkdf2Iter = 32768
	// key length for AES-256
	aesKeyLength = 32
	// hashLength is length of file hash.
	hashLength = 32
)

var (
	// nameRegexp is regular expression to check encrypted name template.
	nameRegexp = regexp.MustCompile(fmt.Sprintf("^[0-9a-f]{%d}$", hashLength*2))
)

// Item is base data struct for incoming data.
type Item struct {
	ID      int64
	Name    string
	Path    string
	Salt    string
	Hash    string
	Counter int
	Created time.Time
	Expired time.Time
}

// InTransaction runs method f and does commit or rollback.
func InTransaction(db *sql.DB, f func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	err = f(tx)
	if err != nil {
		e := tx.Rollback()
		if e != nil {
			err = fmt.Errorf("transaction error=%v, rollback error=%v", err, e)
		}
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
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

func (item *Item) encryptName(key []byte) error {
	if item.Name == "" {
		return errors.New("encrypt empty name")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil
	}
	plainText := []byte(item.Name)
	cipherText := make([]byte, aes.BlockSize+len(plainText))
	iv := cipherText[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return errors.New("iv random generation error")
	}
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(cipherText[aes.BlockSize:], plainText)
	item.Name = hex.EncodeToString(cipherText)
	return nil
}

func (item *Item) decryptName(key []byte) error {
	if item.Name == "" {
		return errors.New("decrypt empty name")
	}
	cipherText, err := hex.DecodeString(item.Name)
	if err != nil {
		return err
	}
	if len(cipherText) < aes.BlockSize {
		return errors.New("invalid cipher block length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return errors.New("new cipher creation")
	}
	iv := cipherText[:aes.BlockSize]
	cipherText = cipherText[aes.BlockSize:]
	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(cipherText, cipherText)
	item.Name = string(cipherText)
	return nil
}

// Encrypt encrypts source file and fills the item by result.
func (item *Item) Encrypt(inFile io.Reader, secret string, l *log.Logger) error {
	salt := make([]byte, saltSize)
	_, err := rand.Read(salt)
	if err != nil {
		return err
	}
	key, keyHash := Key(secret, salt)
	err = item.encryptName(key)
	if err != nil {
		return err
	}
	item.Hash = hex.EncodeToString(keyHash)
	// it is to be called after encryptName
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
	err := item.decryptName(key)
	if err != nil {
		return err
	}
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
	httpWriter, ok := w.(http.ResponseWriter)
	if ok {
		httpWriter.Header().Set(
			"Content-disposition",
			fmt.Sprintf("attachment; filename=\"%v\"", item.Name),
		)
		httpWriter.Header().Set("Content-Type", item.ContentType())
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
	return InTransaction(db, func(tx *sql.Tx) error {
		stmt, err := tx.Prepare("INSERT INTO `storage` (`name`, `path`, `hash`, `salt`, `counter`, `created`, `updated`, `expired`) VALUES (?, ?, ?, ?, ?, ?, ?, ?);")
		if err != nil {
			return err
		}
		r, err := stmt.Exec(item.Name, item.Path, item.Hash, item.Salt, item.Counter, item.Created, item.Created, item.Expired)
		if err != nil {
			return err
		}
		id, err := r.LastInsertId()
		if err != nil {
			return err
		}
		item.ID = id
		return stmt.Close()
	})
}

// Decrement updates items' counter. The first returned parameter is "updated" flags.
func (item *Item) Decrement(db *sql.DB, le *log.Logger) (bool, error) {
	counter := item.Counter
	err := InTransaction(db, func(tx *sql.Tx) error {
		stmt, err := tx.Prepare("UPDATE `storage` SET `counter`=`counter`-1, `updated`=? WHERE `counter`>0 AND `id`=?;")
		if err != nil {
			return err
		}
		defer func() {
			if err := stmt.Close(); err != nil {
				le.Printf("failed close stmt: %v\n", err)
			}
		}()
		_, err = stmt.Exec(time.Now().UTC(), item.ID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return err
		}
		item.Counter--
		return nil
	})
	if err != nil {
		return false, err
	}
	return counter != item.Counter, nil
}

// Delete removes items from database and related file from file system.
func (item *Item) Delete(db *sql.DB, le *log.Logger) error {
	e := InTransaction(db, func(tx *sql.Tx) error {
		// delete an item
		_, err := deleteByIDs(tx, le, item.ID)
		if err != nil {
			return err
		}
		return nil
	})
	if e != nil {
		return fmt.Errorf("failed item delete by id: %v", e)
	}
	return os.Remove(item.FullPath())
}

// IsNameHash checks name can be an encrypted file name.
func IsNameHash(name string) bool {
	return nameRegexp.MatchString(name)
}

// Key calculates and returns secret key and its SHA512 hash.
func Key(secret string, salt []byte) ([]byte, []byte) {
	key := pbkdf2.Key([]byte(secret), salt, pbkdf2Iter, aesKeyLength, sha3.New512)
	b := make([]byte, hashLength)
	sha3.ShakeSum256(b, append(key, salt...))
	return key, b
}

// Read reads an item by its hash from database.
func Read(db *sql.DB, hash string, le *log.Logger) (*Item, error) {
	stmt, err := db.Prepare("SELECT `id`, `name`, `path`, `hash`, `salt`, `counter`, `created`, `expired` FROM `storage` WHERE `counter`>0 AND `hash`=?;")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			le.Printf("failed close stmt: %v\n", err)
		}
	}()
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
	return item, nil
}

// deleteByIDs removes items by their identifiers.
func deleteByIDs(tx *sql.Tx, le *log.Logger, ids ...int64) (int64, error) {
	stmt, err := tx.Prepare("DELETE FROM `storage` WHERE `id` IN (?);")
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			le.Printf("failed close stmt: %v\n", err)
		}
	}()
	strIDs := make([]string, len(ids))
	for i, v := range ids {
		strIDs[i] = strconv.FormatInt(v, 10)
	}
	result, err := stmt.Exec(strings.Join(strIDs, ","))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func deleteByDate(db *sql.DB, le *log.Logger) (int64, error) {
	var n int64
	err := InTransaction(db, func(tx *sql.Tx) error {
		var (
			paths []string
			ids   []int64
		)
		stmt, e := tx.Prepare("SELECT `id`, `path`, `hash` FROM `storage` WHERE `expired`<?;")
		if e != nil {
			return e
		}
		defer func() {
			if err := stmt.Close(); err != nil {
				le.Printf("failed close stmt: %v\n", err)
			}
		}()
		rows, e := stmt.Query(time.Now().UTC())
		if e != nil {
			return e
		}
		item := &Item{} // use only one item to collect paths
		for rows.Next() {
			e = rows.Scan(&item.ID, &item.Path, &item.Hash)
			if e != nil {
				return e
			}
			paths = append(paths, item.FullPath())
			ids = append(ids, item.ID)
		}
		e = rows.Close()
		if e != nil {
			return e
		}
		// delete items from db
		n, e = deleteByIDs(tx, le, ids...)
		if e != nil {
			return e
		}
		// delete files
		for _, p := range paths {
			e = os.RemoveAll(p)
			if e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

// GCMonitor is garbage collection monitoring to delete expired by date or counter items.
func GCMonitor(ch <-chan *Item, closed chan struct{}, db *sql.DB, li, le *log.Logger, period time.Duration) {
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
		case <-closed:
			li.Println("gc monitor stopped")
			return
		}
	}
}
