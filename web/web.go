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
	"io"
	"net/http"

	"github.com/z0rr0/unigma/conf"
)

// Index is a index page HTTP handler.
func Index(w io.Writer, _ *http.Request, cfg *conf.Cfg) (int, error) {
	tpl := cfg.Templates["index"]
	err := tpl.Execute(w, map[string]int{"MaxSize": cfg.Settings.Size})
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}
