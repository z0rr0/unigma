// Copyright 2018 Alexander Zaytsev <me@axv.email>.
// All rights reserved. Use of this source code is governed
// by a MIT-style license that can be found in the LICENSE file.

// Package conf implements methods setup configuration settings.
package conf

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// settings is app settings.
type settings struct {
	TTL   int `json:"ttl"`
	Times int `json:"times"`
}

// Cfg is configuration settings.
type Cfg struct {
	DbSource string   `json:"db"`
	Host     string   `json:"host"`
	Port     uint     `json:"port"`
	Timeout  int64    `json:"timeout"`
	Secure   bool     `json:"secure"`
	Settings settings `json:"settings"`
	Db       *sql.DB
}

// isValid checks the settings are valid.
func (c *Cfg) isValid() error {
	if c.Timeout < 1 {
		return errors.New("invalid timeout value")
	}
	if c.Port < 1 {
		return errors.New("port should be positive")
	}
	if c.Settings.TTL < 1 {
		return errors.New("ttl setting should be positive")
	}
	if c.Settings.Times < 1 {
		return errors.New("times setting should be positive")
	}
	return nil
}

// Addr returns service's net address.
func (c *Cfg) Addr() string {
	return net.JoinHostPort(c.Host, fmt.Sprint(c.Port))
}

// Close frees resources.
func (c *Cfg) Close() error {
	return c.Db.Close()
}

// New returns new configuration.
func New(filename string) (*Cfg, error) {
	fullPath, err := filepath.Abs(strings.Trim(filename, " "))
	if err != nil {
		return nil, err
	}
	_, err = os.Stat(fullPath)
	if err != nil {
		return nil, err
	}
	jsonData, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}
	c := &Cfg{}
	err = json.Unmarshal(jsonData, c)
	if err != nil {
		return nil, err
	}
	err = c.isValid()
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", c.DbSource)
	if err != nil {
		return nil, err
	}
	c.Db = db
	return c, nil
}
