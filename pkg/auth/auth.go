/*
Copyright 2011 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package auth implements Camlistore authentication.
package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"

	"camlistore.org/pkg/netutil"
)

// Operation represents a bitmask of operations. See the OpX constants.
type Operation int

const (
	OpUpload Operation = 1 << iota
	OpStat
	OpGet
	OpEnumerate
	OpRemove
	OpSign
	OpDiscovery
	OpRead   = OpEnumerate | OpStat | OpGet | OpDiscovery
	OpRW     = OpUpload | OpEnumerate | OpStat | OpGet // Not Remove
	OpVivify = OpUpload | OpStat | OpGet | OpDiscovery
	OpAll    = OpUpload | OpEnumerate | OpStat | OpRemove | OpGet | OpSign | OpDiscovery
)

var kBasicAuthPattern = regexp.MustCompile(`^Basic ([a-zA-Z0-9\+/=]+)`)

var (
	mode AuthMode // the auth logic depending on the choosen auth mechanism
)

// An AuthMode is the interface implemented by diffent authentication
// schemes.
type AuthMode interface {
	// AllowedAccess returns a bitmask of all operations
	// this user/request is allowed to do.
	AllowedAccess(req *http.Request) Operation
	// AddAuthHeader inserts in req the credentials needed
	// for a client to authenticate.
	AddAuthHeader(req *http.Request)
}

// UnauthorizedSender may be implemented by AuthModes which want to
// handle sending unauthorized.
type UnauthorizedSender interface {
	// SendUnauthorized sends an unauthorized response,
	// and returns whether it handled it.
	SendUnauthorized(http.ResponseWriter, *http.Request) (handled bool)
}

func FromEnv() (AuthMode, error) {
	return FromConfig(os.Getenv("CAMLI_AUTH"))
}

// An AuthConfigParser parses a registered authentication type's option
// and returns an AuthMode.
type AuthConfigParser func(arg string) (AuthMode, error)

var authConstructor = map[string]AuthConfigParser{
	"none":      newNoneAuth,
	"localhost": newLocalhostAuth,
	"userpass":  newUserPassAuth,
	"devauth":   newDevAuth,
}

// RegisterAuth registers a new authentication scheme.
func RegisterAuth(name string, ctor AuthConfigParser) {
	if _, dup := authConstructor[name]; dup {
		panic("Dup registration of auth mode " + name)
	}
	authConstructor[name] = ctor
}

func newNoneAuth(string) (AuthMode, error) {
	return None{}, nil
}

func newLocalhostAuth(string) (AuthMode, error) {
	return Localhost{}, nil
}

func newDevAuth(pw string) (AuthMode, error) {
	// the vivify mode password is automatically set to "vivi" + Password
	return &DevAuth{pw, "vivi" + pw}, nil
}

func newUserPassAuth(arg string) (AuthMode, error) {
	pieces := strings.Split(arg, ":")
	if len(pieces) < 2 {
		return nil, fmt.Errorf("Wrong userpass auth string; needs to be \"userpass:user:password\"")
	}
	username := pieces[0]
	password := pieces[1]
	mode := &UserPass{Username: username, Password: password}
	for _, opt := range pieces[2:] {
		switch {
		case opt == "+localhost":
			mode.OrLocalhost = true
		case strings.HasPrefix(opt, "vivify="):
			// optional vivify mode password: "userpass:joe:ponies:vivify=rainbowdash"
			mode.VivifyPass = strings.Replace(opt, "vivify=", "", -1)
		default:
			return nil, fmt.Errorf("Unknown userpass option %q", opt)
		}
	}
	return mode, nil
}

// ErrNoAuth is returned when there is no configured authentication.
var ErrNoAuth = errors.New("auth: no configured authentication")

// FromConfig parses authConfig and accordingly sets up the AuthMode
// that will be used for all upcoming authentication exchanges. The
// supported modes are UserPass and DevAuth. UserPass requires an authConfig
// of the kind "userpass:joe:ponies".
//
// If the input string is empty, the error will be ErrNoAuth.
func FromConfig(authConfig string) (AuthMode, error) {
	if authConfig == "" {
		return nil, ErrNoAuth
	}
	pieces := strings.SplitN(authConfig, ":", 2)
	if len(pieces) < 1 {
		return nil, fmt.Errorf("Invalid auth string: %q", authConfig)
	}
	authType := pieces[0]

	if fn, ok := authConstructor[authType]; ok {
		arg := ""
		if len(pieces) == 2 {
			arg = pieces[1]
		}
		return fn(arg)
	}
	return nil, fmt.Errorf("Unknown auth type: %q", authType)
}

// SetMode sets the authentication mode for future requests.
func SetMode(m AuthMode) {
	mode = m
}

func basicAuth(req *http.Request) (string, string, error) {
	auth := req.Header.Get("Authorization")
	if auth == "" {
		return "", "", fmt.Errorf("Missing \"Authorization\" in header")
	}
	matches := kBasicAuthPattern.FindStringSubmatch(auth)
	if len(matches) != 2 {
		return "", "", fmt.Errorf("Bogus Authorization header")
	}
	encoded := matches[1]
	enc := base64.StdEncoding
	decBuf := make([]byte, enc.DecodedLen(len(encoded)))
	n, err := enc.Decode(decBuf, []byte(encoded))
	if err != nil {
		return "", "", err
	}
	pieces := strings.SplitN(string(decBuf[0:n]), ":", 2)
	if len(pieces) != 2 {
		return "", "", fmt.Errorf("didn't get two pieces")
	}
	return pieces[0], pieces[1], nil
}

// UserPass is used when the auth string provided in the config
// is of the kind "userpass:username:pass"
// Possible options appended to the config string are
// "+localhost" and "vivify=pass", where pass will be the
// alternative password which only allows the vivify operation.
type UserPass struct {
	Username, Password string
	OrLocalhost        bool // if true, allow localhost ident auth too
	// Alternative password used (only) for the vivify operation.
	// It is checked when uploading, but Password takes precedence.
	VivifyPass string
}

func (up *UserPass) AllowedAccess(req *http.Request) Operation {
	user, pass, err := basicAuth(req)
	if err == nil {
		if user == up.Username {
			if pass == up.Password {
				return OpAll
			}
			if pass == up.VivifyPass {
				return OpVivify
			}
		}
	}

	if up.OrLocalhost && localhostAuthorized(req) {
		return OpAll
	}

	return 0
}

func (up *UserPass) AddAuthHeader(req *http.Request) {
	req.SetBasicAuth(up.Username, up.Password)
}

type None struct{}

func (None) AllowedAccess(req *http.Request) Operation {
	return OpAll
}

func (None) AddAuthHeader(req *http.Request) {
	// Nothing.
}

type Localhost struct {
	None
}

func (Localhost) AllowedAccess(req *http.Request) (out Operation) {
	if localhostAuthorized(req) {
		return OpAll
	}
	return 0
}

// DevAuth is used for development.  It has one password and one vivify password, but
// also accepts all passwords from localhost. Usernames are ignored.
type DevAuth struct {
	Password string
	// Password for the vivify mode, automatically set to "vivi" + Password
	VivifyPass string
}

func (da *DevAuth) AllowedAccess(req *http.Request) Operation {
	_, pass, err := basicAuth(req)
	if err == nil {
		if pass == da.Password {
			return OpAll
		}
		if pass == da.VivifyPass {
			return OpVivify
		}
	}

	// See if the local TCP port is owned by the same non-root user as this
	// server.  This check performed last as it may require reading from the
	// kernel or exec'ing a program.
	if localhostAuthorized(req) {
		return OpAll
	}

	return 0
}

func (da *DevAuth) AddAuthHeader(req *http.Request) {
	req.SetBasicAuth("", da.Password)
}

func localhostAuthorized(req *http.Request) bool {
	uid := os.Getuid()
	from, err := netutil.HostPortToIP(req.RemoteAddr, nil)
	if err != nil {
		return false
	}
	to, err := netutil.HostPortToIP(req.Host, from)
	if err != nil {
		return false
	}

	// If our OS doesn't support uid.
	// TODO(bradfitz): netutil on OS X uses "lsof" to figure out
	// ownership of tcp connections, but when fuse is mounted and a
	// request is outstanding (for instance, a fuse request that's
	// making a request to camlistored and landing in this code
	// path), lsof then blocks forever waiting on a lock held by the
	// VFS, leading to a deadlock.  Instead, on darwin, just trust
	// any localhost connection here, which is kinda lame, but
	// whatever.  Macs aren't very multi-user anyway.
	if uid == -1 || runtime.GOOS == "darwin" {
		return from.IP.IsLoopback() && to.IP.IsLoopback()
	}

	if uid > 0 {
		owner, err := netutil.AddrPairUserid(from, to)
		if err == nil && owner == uid {
			return true
		}
	}
	return false
}

func isLocalhost(addrPort net.IP) bool {
	return addrPort.IsLoopback()
}

func IsLocalhost(req *http.Request) bool {
	return localhostAuthorized(req)
}

// TODO(mpl): if/when we ever need it:
// func AllowedWithAuth(am AuthMode, req *http.Request, op Operation) bool

// Allowed returns whether the given request
// has access to perform all the operations in op.
func Allowed(req *http.Request, op Operation) bool {
	if op|OpUpload != 0 {
		// upload (at least from camput) requires stat and get too
		op = op | OpVivify
	}
	return mode.AllowedAccess(req)&op == op
}

func TriedAuthorization(req *http.Request) bool {
	// Currently a simple test just using HTTP basic auth
	// (presumably over https); may expand.
	return req.Header.Get("Authorization") != ""
}

func SendUnauthorized(rw http.ResponseWriter, req *http.Request) {
	if us, ok := mode.(UnauthorizedSender); ok {
		if us.SendUnauthorized(rw, req) {
			return
		}
	}
	realm := "camlistored"
	if devAuth, ok := mode.(*DevAuth); ok {
		realm = "Any username, password is: " + devAuth.Password
	}
	rw.Header().Set("WWW-Authenticate", fmt.Sprintf("Basic realm=%q", realm))
	rw.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(rw, "<html><body><h1>Unauthorized</h1>")
}

type Handler struct {
	http.Handler
}

// ServeHTTP serves only if this request and auth mode are allowed all Operations.
func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.serveHTTPForOp(w, r, OpAll)
}

// serveHTTPForOp serves only if op is allowed for this request and auth mode.
func (h Handler) serveHTTPForOp(w http.ResponseWriter, r *http.Request, op Operation) {
	if Allowed(r, op) {
		h.Handler.ServeHTTP(w, r)
	} else {
		SendUnauthorized(w, r)
	}
}

// requireAuth wraps a function with another function that enforces
// HTTP Basic Auth and checks if the operations in op are all permitted.
func RequireAuth(handler func(http.ResponseWriter, *http.Request), op Operation) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		if Allowed(req, op) {
			handler(rw, req)
		} else {
			SendUnauthorized(rw, req)
		}
	}
}
