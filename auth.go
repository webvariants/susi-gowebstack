package main

import (
	"encoding/base64"
	"flag"
	"net/http"
	"strings"
)

var user = flag.String("username", "", "username for basic auth")
var pass = flag.String("password", "", "password for basic auth")

type handler func(w http.ResponseWriter, r *http.Request)

func BasicAuth(pass handler) handler {

	return func(w http.ResponseWriter, r *http.Request) {
		if len(r.Header["Authorization"]) < 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Beware! Protected REALM! "`)
			w.WriteHeader(401)
			w.Write([]byte("401 Unauthorized\n"))
			return
		}
		auth := strings.SplitN(r.Header["Authorization"][0], " ", 2)

		if len(auth) != 2 || auth[0] != "Basic" {
			http.Error(w, "bad syntax", http.StatusBadRequest)
			return
		}

		payload, _ := base64.StdEncoding.DecodeString(auth[1])
		pair := strings.SplitN(string(payload), ":", 2)

		if len(pair) != 2 || !Validate(pair[0], pair[1]) {
			http.Error(w, "authorization failed", http.StatusUnauthorized)
			return
		}

		pass(w, r)
	}
}

func Validate(username, password string) bool {
	if username == *user && password == *pass {
		return true
	}
	return false
}
