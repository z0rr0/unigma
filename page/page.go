// Copyright 2019 Alexander Zaytsev <me@axv.email>.
// All rights reserved. Use of this source code is governed
// by a MIT-style license that can be found in the LICENSE file.

// Package page contains HTML text templates.
package page

const (
	// Index is index page HTML template.
	Index = `
<!DOCTYPE html>
<html>
	<head>
		<meta charset=utf-8>
		<title>Unigma</title>
	</head>
	<body>
		<h1>Unigma</h1>
		<form method="POST" action="/upload/" enctype="multipart/form-data">
			File <small>(max {{.MaxSize}} Mb)</small>: 
			<input type="file" name="file" required>
			TTL: <select name="ttl" required>
				<option value='600'>10 minutes</option>
				<option value='3600'>a hour</option>
				<option value='86400' selected>a day</option>
				<option value='604800'>a week</option>
			</select>
			times: <input type="number" name="times" min="1" max="1000" value="1" required>
			password: <input type="password" name="password" placeholder="secret" required>
			<input type="submit" value="Submit">
		</form>
		<p>
			<small><a href="https://github.com/z0rr0/unigma" title="github.com/z0rr0/enigma">github.com</a></small>
		</p>
	</body>
</html>
`
)
