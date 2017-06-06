/*
 * Copyright (c) 2017 Kurt Jung (Gmail: kurt.w.jung)
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
	"bytes"
	"errors"
	"net/http"
	"net/http/cgi"
	"path"
	"path/filepath"
	"strings"

	"github.com/mholt/caddy/caddyhttp/httpserver"
)

// match returns true if the request string (reqStr) matches the pattern string
// (patternStr), false otherwise. If true is returned, it is followed by the
// prefix that matches the pattern and the unmatched portion to its right.
// patternStr uses glob notation; see path/Match for matching details. If the
// pattern is invalid (for example, contains an unpaired "["), false is
// returned.
func match(reqStr, patternStr string) (ok bool, prefixStr, suffixStr string) {
	var str, last string
	var err error
	str = reqStr
	last = ""
	for last != str && !ok && err == nil {
		ok, err = path.Match(patternStr, str)
		if err == nil {
			if !ok {
				last = str
				str = filepath.Dir(str)
			}
		}
	}
	if ok && err == nil {
		return true, str, reqStr[len(str):]
	}
	return false, "", ""
}

// currentDir returns the current working directory
func currentDir() (wdStr string) {
	wdStr, _ = filepath.Abs(".")
	return
}

// setupCall instantiates a CGI handler based on the incoming request and the
// configuration rule that it matches.
func setupCall(h handlerType, rule ruleType, lfStr, rtStr string,
	rep httpserver.Replacer, hdr http.Header, username string) (cgiHnd cgi.Handler) {
	cgiHnd.Root = "/"
	cgiHnd.Dir = h.root
	rep.Set("root", h.root)
	rep.Set("match", lfStr)
	rep.Set(".", currentDir())
	cgiHnd.Path = rep.Replace(rule.exe)
	cgiHnd.Env = append(cgiHnd.Env, "REMOTE_USER="+username)
	envAdd := func(key, val string) {
		val = rep.Replace(val)
		cgiHnd.Env = append(cgiHnd.Env, key+"="+val)
	}
	// 	if r.TLS != nil {
	// 		env["HTTPS"] = "on"
	// 	}
	for _, env := range rule.envs {
		envAdd(env[0], env[1])
	}
	for _, env := range rule.envs {
		envAdd(env[0], env[1])
	}
	envAdd("PATH_INFO", rtStr)
	envAdd("SCRIPT_FILENAME", cgiHnd.Path)
	envAdd("SCRIPT_NAME", lfStr)
	// Convey JSON Web Token claims to CGI app by means of environment
	for key, list := range hdr {
		if strings.HasPrefix(key, "Token-Claim-") {
			cgiHnd.Env = append(cgiHnd.Env, strings.ToUpper(key)+"="+
				strings.Join(list, "\t"))
		}
	}
	cgiHnd.InheritEnv = append(cgiHnd.InheritEnv, rule.passEnvs...)
	cgiHnd.InheritEnv = append(cgiHnd.InheritEnv, rule.passEnvs...)
	for _, str := range rule.args {
		cgiHnd.Args = append(cgiHnd.Args, rep.Replace(str))
	}
	envAdd("SCRIPT_EXEC", trim(sprintf("%s %s", cgiHnd.Path, join(cgiHnd.Args, " "))))
	return
}

// ServeHTTP satisfies the httpserver.Handler interface.
func (h handlerType) ServeHTTP(w http.ResponseWriter, r *http.Request) (code int, err error) {
	rep := httpserver.NewReplacer(r, nil, "")
	for _, rule := range h.rules {
		for _, matchStr := range rule.matches {
			ok, lfStr, rtStr := match(r.URL.Path, matchStr)
			if ok {
				var buf bytes.Buffer
				// Retrieve name of remote user that was set by some downstream middleware,
				// possibly basicauth.
				remoteUser, _ := r.Context().Value(httpserver.RemoteUserCtxKey).(string) // Blank if not set
				cgiHnd := setupCall(h, rule, lfStr, rtStr, rep, r.Header, remoteUser)
				cgiHnd.Stderr = &buf
				cgiHnd.ServeHTTP(w, r)
				if buf.Len() > 0 {
					err = errors.New(trim(buf.String()))
				}
				return
			}
		}
	}
	return h.next.ServeHTTP(w, r)
}
