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
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite3 driver package
	"github.com/z0rr0/unigma/db"
	"github.com/z0rr0/unigma/page"
)

// settings is app settings.
type settings struct {
	TTL   int `json:"ttl"`
	Times int `json:"times"`
	Size  int `json:"size"`
}

// Cfg is configuration settings.
type Cfg struct {
	DbSource   string   `json:"db"`
	Storage    string   `json:"storage"`
	Host       string   `json:"host"`
	Port       uint     `json:"port"`
	Timeout    int64    `json:"timeout"`
	Secure     bool     `json:"secure"`
	Salt       string   `json:"salt"`
	GCPeriod   int64    `json:"gc_period"`
	Settings   settings `json:"settings"`
	StorageDir string
	Db         *sql.DB
	Templates  map[string]*template.Template
	ErrLogger  *log.Logger
	timeout    time.Duration
	Ch         chan *db.Item
}

// isValid checks the settings are valid.
func (c *Cfg) isValid() error {
	fullPath, err := filepath.Abs(strings.Trim(c.Storage, " "))
	if err != nil {
		return err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("storage is not a directory")
	}
	mode := uint(info.Mode().Perm())
	if mode&uint(0600) != 0600 {
		return errors.New("storage dir is not writable or readable")
	}
	c.StorageDir = fullPath

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
	if c.Settings.Size < 1 {
		return errors.New("size setting should be positive")
	}
	if c.GCPeriod < 1 {
		return errors.New("gc_period should be positive")
	}
	err = c.loadTemplates()
	if err != nil {
		return err
	}
	c.timeout = time.Duration(c.Timeout) * time.Second
	c.Ch = make(chan *db.Item, 1)
	return nil
}

// loadTemplates loads HTML templates to memory.
func (c *Cfg) loadTemplates() error {
	if len(c.Templates) > 0 {
		return errors.New("templates are already loaded")
	}
	pages := map[string]string{
		"index":  page.Index,
		"error":  page.Error,
		"result": page.Result,
		"read":   page.Read,
	}
	c.Templates = make(map[string]*template.Template, len(pages))
	for name, content := range pages {
		tpl, err := template.New(name).Parse(content)
		if err != nil {
			return err
		}
		c.Templates[name] = tpl
	}
	return nil
}

// Addr returns service's net address.
func (c *Cfg) Addr() string {
	return net.JoinHostPort(c.Host, fmt.Sprint(c.Port))
}

// HandleTimeout is service timeout.
func (c *Cfg) HandleTimeout() time.Duration {
	return c.timeout
}

// MaxFileSize return max file size.
func (c *Cfg) MaxFileSize() int {
	return c.Settings.Size << 20
}

// Close frees resources.
func (c *Cfg) Close() error {
	close(c.Ch)
	return c.Db.Close()
}

// Secret returns secret string.
func (c *Cfg) Secret(p string) string {
	return p + c.Salt
}

// New returns new configuration.
func New(filename string, l *log.Logger) (*Cfg, error) {
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
	odb, err := sql.Open("sqlite3", c.DbSource)
	if err != nil {
		return nil, err
	}
	c.Db = odb
	c.ErrLogger = l
	return c, nil
}
