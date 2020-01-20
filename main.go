package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/erikfastermann/chat/db"
	"github.com/erikfastermann/chat/handler"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	type entry struct {
		name string
		dest *string
	}

	var addr, https, domain, cert, key string
	var dbDir, tmplt string
	for _, e := range []entry{
		{"ADDRESS", &addr},
		{"HTTPS_ADDRESS", &https},
		{"DOMAIN", &domain},
		{"CERT", &cert},
		{"KEY", &key},

		{"DB_DIR", &dbDir},
		{"TEMPLATE_GLOB", &tmplt},
	} {
		env := "CHAT_" + e.name
		*e.dest = os.Getenv(env)
		if *e.dest == "" {
			return fmt.Errorf("env %s is empty or unset", env)
		}
	}

	ws, err := url.Parse(domain)
	if err != nil {
		return err
	}
	ws.Scheme = "wss"

	h := &handler.Handler{
		WebSocket: ws.String(),
	}
	h.DB, err = db.Open(dbDir)
	if err != nil {
		return err
	}
	defer h.DB.Close()

	h.Templates, err = template.ParseGlob(tmplt)
	if err != nil {
		return err
	}

	go func() {
		srv := newServer(addr, http.RedirectHandler(domain, http.StatusMovedPermanently))
		log.Fatal(srv.ListenAndServe())
	}()

	srv := newServer(https, h)
	log.Printf("server: listening on address %q (https)", https)
	log.Printf("server: redirecting http (address: %q) to %q", addr, domain)
	return srv.ListenAndServeTLS(cert, key)
}

func newServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:           addr,
		Handler:        h,
		MaxHeaderBytes: 1 << 20,
	}
}
