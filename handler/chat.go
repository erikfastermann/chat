package handler

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/erikfastermann/chat/db"
	"golang.org/x/net/websocket"
)

const (
	queryRoom = "room"
	queryWSS  = "wss"
)

func (h *Handler) addRoom(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return errMethod(r.Method)
	}

	room := r.PostFormValue(queryRoom)
	if err := h.DB.AddRoom(room); err != nil {
		if errors.Is(err, db.ErrInvalidName) || errors.Is(err, db.ErrExists) {
			return badRequest(err)
		}
		return err
	}

	http.Redirect(w, r, fmt.Sprintf("%s?%s=%s", routeChat, queryRoom, room), http.StatusSeeOther)
	return nil
}

func (h *Handler) chat(username string, w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		return errMethod(r.Method)
	}

	room := r.FormValue(queryRoom)
	chat, err := h.DB.Join(room)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return badRequest(err)
		}
		return err
	}

	if r.FormValue(queryWSS) == "true" {
		websocket.Server{Handler: func(ws *websocket.Conn) {
			go func() {
				r := bufio.NewReader(ws)
				for {
					content, err := r.ReadString('\n')
					if err != nil {
						if err != io.EOF {
							log.Print(err)
						}
						return
					}
					chat.Send <- &db.Msg{
						Author:  username,
						Date:    time.Now(),
						Content: content[:len(content)-1],
					}
				}
			}()

			log.Print(chat.Listen(func(m *db.Msg) error {
				if err := json.NewEncoder(ws).Encode(m); err != nil {
					return err
				}
				return nil
			}))
		}}.ServeHTTP(w, r)

		return nil
	}

	return h.Templates.ExecuteTemplate(w, templateChat, struct {
		Username  string
		Current   string
		Chat      []*db.Msg
		Rooms     []string
		WebSocket string
	}{
		Username:  username,
		Current:   room,
		Chat:      chat.Latest(0, -1),
		Rooms:     h.DB.Rooms(room),
		WebSocket: fmt.Sprintf("%s/?%s=%s&%s=true", h.WebSocket, queryRoom, room, queryWSS),
	})
}
