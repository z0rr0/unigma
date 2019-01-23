package db

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

const (
	saltSize   = 64
	pbkdf2Iter = 4096
)

type Item struct {
	Name    string
	Path    string
	Hash    string
	Counter int
	Created time.Time
	Expired time.Time
}

func Encrypt(inFile io.Reader, item *Item, secret string, l *log.Logger) error {
	const aesKeyLength = 32

	salt := make([]byte, saltSize)
	_, err := rand.Read(salt)
	if err != nil {
		return err
	}
	item.Hash = hex.EncodeToString(salt) // public hash based on random salt
	key := pbkdf2.Key([]byte(secret), salt, pbkdf2Iter, aesKeyLength, sha512.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	// the key is unique for each ciphertext, then it's ok to use a zero IV.
	var iv [aes.BlockSize]byte
	stream := cipher.NewOFB(block, iv[:])

	item.Path = filepath.Join(item.Path, item.Hash)
	_, err = os.Stat(item.Path)
	if err == nil {
		return fmt.Errorf("not unique file name: %v", item.Path)
	}
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
	// Copy the input file to the output file, encrypting as we go.
	if _, err := io.Copy(writer, inFile); err != nil {
		return err
	}
	return nil
}
