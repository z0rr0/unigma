package db

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// saltSize is random salt, also used for storage file name
	saltSize = 32
	// pbkdf2Iter is number of pbkdf2 iterations
	pbkdf2Iter = 4096
	// key length for AES-256
	aesKeyLength = 32
)

// Item is base data struct for incoming data.
type Item struct {
	Name    string
	Path    string
	Salt    string
	Hash    string
	Counter int
	Created time.Time
	Expired time.Time
}

// genSalt generates unique random salt to decrease collisions.
func (item *Item) genSalt() ([]byte, error) {
	const attempts = 16
	var fileName, hash string

	salt := make([]byte, saltSize)
	for i := 0; i < attempts; i++ {
		_, err := rand.Read(salt)
		if err != nil {
			return nil, err
		}
		hash = hex.EncodeToString(salt)
		fileName = filepath.Join(item.Path, hash)
		_, err = os.Stat(fileName)
		if (err != nil) && os.IsNotExist(err) {
			item.Salt = hash
			item.Path = fileName
			return salt, nil
		}
	}
	return nil, fmt.Errorf("can't generate unique salt after %v attempts", attempts)
}

// setHash calculates and sets has(secret+salt) hash.
func (item *Item) setHash(secret string, salt []byte) {
	h := sha512.New512_256()
	b := []byte(secret)
	b = append(b, salt...)
	// to check secrete without decryption
	item.Hash = hex.EncodeToString(h.Sum(b))
}

// checkHash compares hash(secret+salt) with item's value.
func (item *Item) checkHash(secret string) ([]byte, error) {
	salt, err := hex.DecodeString(item.Salt)
	if err != nil {
		return nil, err
	}
	hash, err := hex.DecodeString(item.Hash)
	if err != nil {
		return nil, err
	}
	h := sha512.New512_256()
	b := []byte(secret)
	b = append(b, salt...)

	if !hmac.Equal(h.Sum(b), hash) {
		return nil, errors.New("failed secret")
	}
	return salt, nil
}

// Encrypt encrypts source file and fills the item by result.
func (item *Item) Encrypt(inFile io.Reader, secret string, l *log.Logger) error {
	salt, err := item.genSalt()
	if err != nil {
		return err
	}
	item.setHash(secret, salt)

	key := pbkdf2.Key([]byte(secret), salt, pbkdf2Iter, aesKeyLength, sha512.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	// the key is unique for each cipher-text, then it's ok to use a zero IV.
	var iv [aes.BlockSize]byte
	stream := cipher.NewOFB(block, iv[:])

	outFile, err := os.OpenFile(item.Path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
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
func (item *Item) Decrypt(w io.Writer, secret string, l *log.Logger) error {
	salt, err := item.checkHash(secret)
	if err != nil {
		return err
	}
	inFile, err := os.Open(item.Path)
	if err != nil {
		return err
	}
	defer func() {
		if err := inFile.Close(); err != nil {
			l.Printf("close in-encypted file error: %v", err)
		}
	}()
	key := pbkdf2.Key([]byte(secret), salt, pbkdf2Iter, aesKeyLength, sha512.New)
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
