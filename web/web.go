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

func validate(r *http.Request, cfg *conf.Cfg) (*db.Item, string, error) {
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
	return item, password, nil
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

// Save gets incoming upload request and encrypted and save file to the storage.
func Save(w io.Writer, r *http.Request, cfg *conf.Cfg) (int, error) {
	item, pwd, err := validate(r, cfg)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	f, h, err := r.FormFile("file")
	if err != nil {
		return http.StatusInternalServerError, err
	}
	fmt.Println("xaz", f, h)
	defer func() {
		if err := r.Body.Close(); err != nil {
			cfg.ErrLogger.Printf("close body: %v", err)
		}
		if err := f.Close(); err != nil {
			cfg.ErrLogger.Printf("close incoming file: %v", err)
		}
	}()
	item.Name = h.Filename
	err = db.Encrypt(f, item, cfg.Salt+pwd, cfg.ErrLogger)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	fmt.Println("xaz", item)
	return Index(w, r, cfg)
}
