package db

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

type Msg struct {
	Author  string    `json:"author"`
	Date    time.Time `json:"date"`
	Content string    `json:"content"`
}

const (
	msgAuthor = iota
	msgDate
	msgContent
	msgLen
)

const msgTimeFormat = time.RFC3339

func msgFromRec(r []string) (*Msg, error) {
	if len(r) != msgLen {
		return nil, fmt.Errorf(
			"msg csv record: expected item count %d, got %d",
			msgLen,
			len(r),
		)
	}

	t, err := time.Parse(msgTimeFormat, r[msgDate])
	if err != nil {
		return nil, err
	}

	return &Msg{
		Author:  r[msgAuthor],
		Date:    t,
		Content: r[msgContent],
	}, nil
}

func (m *Msg) toRec() []string {
	r := make([]string, msgLen)
	r[msgAuthor] = m.Author
	r[msgDate] = m.Date.Format(msgTimeFormat)
	r[msgContent] = m.Content
	return r
}

func (db *DB) Rooms(exclude string) []string {
	db.mu.RLock()
	defer db.mu.RUnlock()

	rooms := make([]string, 0)
	for r := range db.rooms {
		if r == exclude {
			continue
		}
		rooms = append(rooms, r)
	}
	return rooms
}

func (db *DB) AddRoom(room string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if err := checkRoomName(room); err != nil {
		return err
	}
	if _, ok := db.rooms[room]; ok {
		return fmt.Errorf("room %q: %w", room, ErrExists)
	}

	f, err := db.open(dirRooms, room+".csv")
	if err != nil {
		return err
	}
	r := &Room{f: f}
	db.rooms[room] = r
	r.loop()

	return nil
}

func (db *DB) Join(room string) (*Room, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	r, ok := db.rooms[room]
	if !ok {
		return nil, fmt.Errorf("can't join room %q, %w", room, ErrNotFound)
	}

	return r, nil
}

type Room struct {
	mu sync.RWMutex

	f    *os.File
	msgs []*Msg
	Send chan<- *Msg

	ctr   int
	recvs map[int]chan *Msg
}

const roomWelcome = "Welcome"

func (r *Room) loop() {
	c := make(chan *Msg)
	r.Send = c

	go func() {
		for {
			m := <-c

			r.mu.Lock()
			if err := write(r.f, m.toRec()); err != nil {
				r.mu.Unlock()
				log.Print(err)
				continue
			}
			r.msgs = append(r.msgs, m)
			r.mu.Unlock()

			go func() {
				r.mu.RLock()
				for _, recv := range r.recvs {
					recv <- m
				}
				r.mu.RUnlock()
			}()
		}
	}()
}

func (r *Room) Listen(f func(m *Msg) error) error {
	c := make(chan *Msg)

	r.mu.Lock()
	cur := r.ctr
	if r.recvs == nil {
		r.recvs = make(map[int]chan *Msg)
	}
	r.recvs[cur] = c
	r.ctr++
	r.mu.Unlock()

	var err error
	for m := range c {
		if err = f(m); err != nil {
			break
		}
	}

	r.mu.Lock()
	delete(r.recvs, cur)
	r.mu.Unlock()

	return err
}

func (r *Room) Latest(offset, limit int) []*Msg {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if offset < 0 {
		panic("negative offset")
	}

	l := len(r.msgs)
	if offset >= l {
		return nil
	}
	to := l - offset
	from := to - limit
	if from < 0 || limit < 0 {
		from = 0
	}

	s := r.msgs[from:to]
	latest := make([]*Msg, len(s))
	copy(latest, s)

	return latest
}

func checkRoomName(room string) error {
	if len(room) < 3 {
		return fmt.Errorf("room %q: name too short, %w", room, ErrInvalidName)
	}
	if len(room) > 24 {
		return fmt.Errorf("room %q: name too long, %w", room, ErrInvalidName)
	}

	for _, ch := range room {
		if !((ch >= '0' && ch <= '9') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= 'a' && ch <= 'z') ||
			ch == '.' || ch == '_' || ch == '-') {
			return fmt.Errorf("room %q: %w, not allowed character %q",
				room,
				ErrInvalidName,
				ch,
			)
		}
	}

	return nil
}
