/*
 * Copyright (c) 2017 Kurt Jung (Gmail: kurt.w.jung)
 * Copyright (c) 2020 Andreas Schneider
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package cgi

import (
	"net/http"
	"net/http/cgi"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

// match returns true if the request string (reqStr) matches the pattern string
// (patternStr), false otherwise. If true is returned, it is followed by the
// prefix that matches the pattern and the unmatched portion to its right.
// patternStr uses glob notation; see path/Match for matching details. If the
// pattern is invalid (for example, contains an unpaired "["), false is
// returned.
func match(requestStr string, patterns []string) (ok bool, prefixStr, suffixStr string) {
	var str, last string
	var err error
	ln := len(patterns)
	for j := 0; j < ln && !ok; j++ {
		pattern := patterns[j]
		str = requestStr
		last = ""
		for last != str && !ok && err == nil {
			ok, err = path.Match(pattern, str)
			if err == nil {
				if ok {
					prefixStr = str
					suffixStr = requestStr[len(str):]
				} else {
					last = str
					str = filepath.Dir(str)
				}
			}
		}
	}
	return
}

// excluded returns true if the request string (reqStr) matches any of the
// pattern strings (patterns), false otherwise. patterns use glob notation; see
// path/Match for matching details. If the pattern is invalid (for example,
// contains an unpaired "["), false is returned.
func excluded(reqStr string, patterns []string) (ok bool) {
	var err error
	var match bool

	ln := len(patterns)
	for j := 0; j < ln && !ok; j++ {
		match, err = path.Match(patterns[j], reqStr)
		if err == nil {
			if match {
				ok = true
				// fmt.Printf("[%s] is excluded by rule [%s]\n", reqStr, patterns[j])
			}
		}
	}
	return
}

// currentDir returns the current working directory
func currentDir() (wdStr string) {
	wdStr, _ = filepath.Abs(".")
	return
}

// passAll returns a slice of strings made up of each environment key
func passAll() (list []string) {
	envList := os.Environ() // ["HOME=/home/foo", "LVL=2", ...]
	for _, str := range envList {
		pos := strings.Index(str, "=")
		if pos > 0 {
			list = append(list, str[:pos])
		}
	}
	return
}

func (c CGI) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	// For convenience: get the currently authenticated user; if some other middleware has set that.
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	var username string
	if usernameVal, exists := repl.Get("http.auth.user.id"); exists {
		if usernameVal, ok := usernameVal.(string); ok {
			username = usernameVal
		}
	}

	var cgiHandler cgi.Handler

	cgiHandler.Root = "/"

	cgiHandler.Dir = c.dir
	cgiHandler.Path = repl.ReplaceAll(c.exe, "")
	for _, str := range c.args {
		cgiHandler.Args = append(cgiHandler.Args, repl.ReplaceAll(str, ""))
	}

	envAdd := func(key, val string) {
		val = repl.ReplaceAll(val, "")
		cgiHandler.Env = append(cgiHandler.Env, key+"="+val)
	}
	envAdd("PATH_INFO", r.URL.Path)
	envAdd("SCRIPT_FILENAME", cgiHandler.Path)
	envAdd("SCRIPT_NAME", r.URL.Path) // TODO: split according to matcher?
	cgiHandler.Env = append(cgiHandler.Env, "REMOTE_USER="+username)

	for _, e := range c.envs {
		cgiHandler.Env = append(cgiHandler.Env, repl.ReplaceAll(e, ""))
	}

	if c.passAll {
		cgiHandler.InheritEnv = passAll()
	} else {
		cgiHandler.InheritEnv = append(cgiHandler.InheritEnv, c.passEnvs...)
	}

	if c.inspect {
		inspect(cgiHandler, w, r, repl)
	} else {
		cgiHandler.ServeHTTP(w, r)
	}
	return next.ServeHTTP(w, r)
}
