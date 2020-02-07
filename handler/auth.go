package handler

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net/http"

	"github.com/erikfastermann/chat/db"
	"golang.org/x/crypto/bcrypt"
)

const sessToken = "session_token"

func (h *Handler) checkAuth(r *http.Request) (string, error) {
	c, err := r.Cookie(sessToken)
	if err != nil {
		return "", err
	}

	token := c.Value
	username, err := h.DB.UserByToken(token)
	if err != nil {
		return "", err
	}

	return username, nil
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return errMethod(r.Method)
	}

	username, password := r.FormValue("username"), r.FormValue("password")
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if err := h.DB.AddUser(username, hash); err != nil {
		if errors.Is(err, db.ErrExists) {
			return badRequest(err)
		}
		return err
	}

	http.Redirect(w, r, routeLogin, http.StatusSeeOther)
	return nil
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) error {
	if r.Method == http.MethodGet {
		return h.Templates.ExecuteTemplate(w, templateLogin, nil)
	}

	if r.Method != http.MethodPost {
		return errMethod(r.Method)
	}

	username, password := r.FormValue("username"), r.FormValue("password")
	passHash, err := h.DB.User(username)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			http.Redirect(w, r, routeLogin, http.StatusSeeOther)
			return unauthf("username %q doesn't exist", username)
		}
		return err
	}

	if err := bcrypt.CompareHashAndPassword(passHash, []byte(password)); err != nil {
		http.Redirect(w, r, routeLogin, http.StatusSeeOther)
		return unauthf("username %q: wrong password", username)
	}

	buf := make([]byte, 64)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return err
	}
	token := base64.URLEncoding.EncodeToString(buf)

	if err := h.DB.SetToken(username, token); err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessToken,
		Value:    token,
		Path:     "/",
		Secure:   r.URL.Scheme == "https",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	http.Redirect(w, r, routeRoomWelcome, http.StatusSeeOther)
	return nil
}

func (h *Handler) logout(username string, w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		return errMethod(r.Method)
	}

	if err := h.DB.SetToken(username, ""); err != nil {
		return err
	}

	http.Redirect(w, r, routeLogin, http.StatusSeeOther)
	return nil
}
