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
	"time"

	"github.com/z0rr0/unigma/conf"
	"github.com/z0rr0/unigma/db"
)

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
		return nil, "", errors.New("required field ttl")
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

func validateDownload(r *http.Request, cfg *conf.Cfg) (*db.Item, []byte, error) {
	password := r.PostFormValue("password")
	if password == "" {
		return nil, nil, errors.New("required password")
	}
	item := &db.Item{
		Counter: 1,
		Path:    cfg.StorageDir,
	}
	key, err := item.IsValidSecret(cfg.Secret(password))
	if err != nil {
		return nil, nil, err
	}
	return item, key, nil
}

// Index is a index page HTTP handler.
func Index(w io.Writer, _ *http.Request, cfg *conf.Cfg) (int, error) {
	tpl := cfg.Templates["index"]
	err := tpl.Execute(w, map[string]int{"MaxSize": cfg.Settings.Size})
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

// Upload gets incoming upload request and encrypted and save file to the storage.
func Upload(w io.Writer, r *http.Request, cfg *conf.Cfg) (int, error) {
	item, secret, err := validateUpload(r, cfg)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	f, h, err := r.FormFile("file")
	if err != nil {
		return http.StatusInternalServerError, err
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
		return http.StatusInternalServerError, err
	}
	return Index(w, r, cfg)
}

// Download returns a file.
func Download(w io.Writer, r *http.Request, cfg *conf.Cfg) (int, error) {
	item, key, err := validateDownload(r, cfg)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	httpWriter, ok := w.(http.ResponseWriter)
	if ok {
		httpWriter.Header().Set("Content-disposition", fmt.Sprintf("attachment; filename=\"%v\"", item.Name))
		httpWriter.Header().Set("Content-Type", item.ContentType())
	}
	err = item.Decrypt(w, key, cfg.ErrLogger)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}
