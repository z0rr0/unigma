// Copyright 2019 Alexander Zaytsev <me@axv.email>.
// All rights reserved. Use of this source code is governed
// by a MIT-style license that can be found in the LICENSE file.

// Package web contains HTTP handlers methods.
// There are 2 URLs:
// "/" - GET index page
// "/upload" - POST save file and settings
// "/<hash>" - GET and POST get file
package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/z0rr0/unigma/conf"
	"github.com/z0rr0/unigma/db"
)

// IndexData is a struct for index page init data.
type IndexData struct {
	Err     string
	Msg     string
	MaxSize int
}

// validateRange converts value to integer and checks that it is in a range [1; max].
func validateRange(value, field string, max int) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if (n < 1) || (n > max) {
		return 0, fmt.Errorf("field %v=%v but available range [%v - %v]", field, n, 1, max)
	}
	return n, nil
}

func validateUpload(r *http.Request, cfg *conf.Cfg) (*db.Item, string, error) {
	// TTL
	value := r.PostFormValue("ttl")
	if value == "" {
		return nil, "", errors.New("required field TTL")
	}
	ttl, err := validateRange(value, "ttl", cfg.Settings.TTL)
	if err != nil {
		return nil, "", err
	}
	// times
	value = r.PostFormValue("times")
	if value == "" {
		return nil, "", errors.New("required field times")
	}
	counter, err := validateRange(value, "times", cfg.Settings.Times)
	if err != nil {
		return nil, "", err
	}
	// password
	password := r.PostFormValue("password")
	if password == "" {
		return nil, "", errors.New("required field password")
	}
	now := time.Now().UTC()
	item := &db.Item{
		Counter: counter,
		Path:    cfg.StorageDir,
		Created: now,
		Expired: now.Add(time.Duration(ttl) * time.Second),
	}
	return item, cfg.Secret(password), nil
}

func validateDownload(item *db.Item, r *http.Request, cfg *conf.Cfg) ([]byte, error) {
	password := r.PostFormValue("password")
	if password == "" {
		return nil, errors.New("required password")
	}
	if !item.IsFileExists() {
		return nil, errors.New("file not found")
	}
	key, err := item.IsValidSecret(cfg.Secret(password))
	if err != nil {
		return nil, err
	}
	return key, nil
}

// Error sets error page. It returns code value.
func Error(w io.Writer, cfg *conf.Cfg, code int, msg string, tplName string) int {
	if tplName == "" {
		tplName = "error"
	}
	title := "Error"
	httpWriter, ok := w.(http.ResponseWriter)
	if ok {
		httpWriter.WriteHeader(code)
	}
	switch code {
	case http.StatusNotFound:
		title, msg = "Not found", "Page not found"
	case http.StatusBadRequest:
		if msg == "" {
			msg = "Failed validation data"
		}
	default:
		msg = "Sorry, it is an error"
	}
	tpl := cfg.Templates[tplName]
	err := tpl.Execute(w, &IndexData{Err: title, Msg: msg})
	if err != nil {
		cfg.ErrLogger.Printf("error-template '%v' execute failed: %v\n", tplName, err)
		return http.StatusInternalServerError
	}
	return code
}

// Index is a index page HTTP handler.
func Index(w io.Writer, _ *http.Request, cfg *conf.Cfg) (int, error) {
	tpl := cfg.Templates["index"]
	err := tpl.Execute(w, IndexData{MaxSize: cfg.Settings.Size})
	if err != nil {
		return Error(w, cfg, http.StatusInternalServerError, "", "error"), err
	}
	return http.StatusOK, nil
}

// Upload gets an incoming upload request, encrypts and saves file to the storage.
func Upload(w io.Writer, r *http.Request, cfg *conf.Cfg) (int, error) {
	item, secret, err := validateUpload(r, cfg)
	if err != nil {
		return Error(w, cfg, http.StatusBadRequest, err.Error(), "index"), err
	}
	f, h, err := r.FormFile("file")
	if err != nil {
		return Error(w, cfg, http.StatusBadRequest, "field file is required", "index"), err
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			cfg.ErrLogger.Printf("close body: %v", err)
		}
		if err := f.Close(); err != nil {
			cfg.ErrLogger.Printf("close incoming file: %v", err)
		}
	}()
	item.Name = h.Filename
	err = item.Encrypt(f, secret, cfg.ErrLogger)
	if err != nil {
		return Error(w, cfg, http.StatusInternalServerError, "", ""), err
	}
	err = item.Save(cfg.Db)
	if err != nil {
		return Error(w, cfg, http.StatusInternalServerError, "", ""), err
	}
	tpl := cfg.Templates["result"]
	err = tpl.Execute(w, map[string]string{"URL": item.GetURL(r, cfg.Secure).String()})
	if err != nil {
		return Error(w, cfg, http.StatusInternalServerError, "", ""), err
	}
	return http.StatusOK, nil
}

func readFile(w io.Writer, r *http.Request, item *db.Item, cfg *conf.Cfg) (int, error) {
	key, err := validateDownload(item, r, cfg)
	if err != nil {
		return Error(w, cfg, http.StatusBadRequest, err.Error(), "read"), err
	}
	httpWriter, ok := w.(http.ResponseWriter)
	if ok {
		httpWriter.Header().Set(
			"Content-disposition",
			fmt.Sprintf("attachment; filename=\"%v\"", item.Name),
		)
		httpWriter.Header().Set("Content-Type", item.ContentType())
	}
	// file exists and secret is valid, so decrement counter
	ok, err = item.Decrement(cfg.Db)
	if err != nil {
		return Error(w, cfg, http.StatusInternalServerError, "", "error"), err
	}
	if !ok {
		return Error(w, cfg, http.StatusNotFound, "", ""), nil
	}
	err = item.Decrypt(w, key, cfg.ErrLogger)
	if err != nil {
		return Error(w, cfg, http.StatusInternalServerError, "", "error"), err
	}
	return http.StatusOK, nil
}

// Download returns a decrypted file.
func Download(w io.Writer, r *http.Request, cfg *conf.Cfg) (int, error) {
	hash := strings.Trim(r.RequestURI, "/ ")
	item, err := db.Read(cfg.Db, hash)
	if err != nil {
		return Error(w, cfg, http.StatusInternalServerError, "", ""), err
	}
	if item.ID == 0 {
		return Error(w, cfg, http.StatusNotFound, "", ""), nil
	}
	if r.Method == "POST" {
		return readFile(w, r, item, cfg)
	}
	tpl := cfg.Templates["read"]
	err = tpl.Execute(w, nil)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}
