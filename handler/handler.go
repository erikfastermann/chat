package handler

import (
	"bufio"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"path"

	"github.com/erikfastermann/chat/db"
)

const (
	routeRegister    = "/register"
	routeLogin       = "/login"
	routeLogout      = "/logout"
	routeChat        = "/"
	routeRoomWelcome = "/?room=Welcome"
	routeAddRoom     = "/add"
)

const (
	templateLogin = "login.html"
	templateChat  = "chat.html"
)

type Handler struct {
	WebSocket string
	DB        *db.DB
	Templates *template.Template
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	addr := r.RemoteAddr
	method := r.Method
	path := r.URL.String()
	ww := &writer{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}

	err := h.serve(ww, r)
	if err != nil {
		switch v := err.(type) {
		case errStatus:
			ww.statusCode = v.statusCode
		default:
			ww.statusCode = http.StatusInternalServerError
		}

		if !ww.wroteHdr {
			fmt.Fprintf(w, "%d - %s", ww.statusCode, http.StatusText(ww.statusCode))
		}
	}

	log.Printf("%q|%d - %s|%v",
		fmt.Sprintf("%s|%s %s", addr, method, path),
		ww.statusCode,
		http.StatusText(ww.statusCode),
		err,
	)
}
func (h *Handler) serve(w http.ResponseWriter, r *http.Request) error {
	hdr := w.Header()
	hdr.Add("Referrer-Policy", "no-referrer")
	hdr.Add("X-Frame-Options", "DENY")
	hdr.Add("X-Content-Type-Options", "nosniff")
	hdr.Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains")

	link := path.Clean(r.URL.Path)
	username, loginErr := h.checkAuth(r)
	if loginErr != nil && link != routeRegister && link != routeLogin {
		http.Redirect(w, r, routeLogin, http.StatusSeeOther)
		return unauthf("unauthorized, %v", loginErr)
	}

	switch link {
	case routeRegister:
		if loginErr != nil {
			return h.register(w, r)
		}
		http.Redirect(w, r, routeRoomWelcome, http.StatusSeeOther)
		return nil
	case routeLogin:
		if loginErr != nil {
			return h.login(w, r)
		}
		http.Redirect(w, r, routeRoomWelcome, http.StatusSeeOther)
		return nil
	case routeLogout:
		return h.logout(username, w, r)
	case routeChat:
		return h.chat(username, w, r)
	case routeAddRoom:
		return h.addRoom(w, r)
	default:
		return errStatus{
			statusCode: http.StatusNotFound,
			error:      fmt.Errorf("url %q not found", r.URL),
		}
	}
}

type writer struct {
	http.ResponseWriter
	wroteHdr   bool
	statusCode int
}

func (w *writer) WriteHeader(statusCode int) {
	w.wroteHdr = true
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *writer) Write(p []byte) (int, error) {
	w.wroteHdr = true
	return w.ResponseWriter.Write(p)
}

func (w *writer) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.ResponseWriter.(http.Hijacker).Hijack()
}

type errStatus struct {
	statusCode int
	error
}

func badRequest(err error) error {
	return errStatus{
		statusCode: http.StatusBadRequest,
		error:      err,
	}
}

func unauthf(format string, a ...interface{}) error {
	return errStatus{
		statusCode: http.StatusUnauthorized,
		error:      fmt.Errorf(format, a...),
	}
}

func errMethod(method string) error {
	return errStatus{
		statusCode: http.StatusMethodNotAllowed,
		error:      fmt.Errorf("method %q not allowed", method),
	}
}
