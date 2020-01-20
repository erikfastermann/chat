package db

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrExists      = errors.New("already exists")
	ErrInvalidName = errors.New("invalid name")
)

type DB struct {
	dir string

	users users
	mu    sync.RWMutex
	rooms map[string]*Room
}

const (
	csvUsers = "users.csv"
	dirRooms = "rooms"
)

func Open(dbDir string) (*DB, error) {
	db := &DB{
		dir: dbDir,
		users: users{
			names:  make(map[string]*user),
			tokens: make(map[string]*user),
		},
		rooms: make(map[string]*Room),
	}

	var err error
	db.users.File, err = db.open(csvUsers)
	if err != nil {
		return nil, err
	}

	err = func() error {
		recs, err := csv.NewReader(db.users.File).ReadAll()
		if err != nil {
			return err
		}
		for _, r := range recs {
			u, err := userFromRec(r)
			if err != nil {
				return err
			}
			db.users.names[u.username] = u
		}

		roomsPath := filepath.Join(db.dir, dirRooms)
		roomNames := make([]string, 0)
		d, err := os.Open(roomsPath)
		switch err {
		case nil:
			roomNames, err = d.Readdirnames(0)
			if err != nil {
				return err
			}
		default:
			switch os.IsNotExist(err) {
			case true:
				if err := os.Mkdir(roomsPath, 0755); err != nil {
					return err
				}
			default:
				return err
			}
		}

		for _, n := range roomNames {
			ext := filepath.Ext(n)
			room := n[:len(n)-len(ext)]
			if ext != ".csv" || len(room) < 1 {
				return fmt.Errorf("invalid file name %q", n)
			}

			if err := checkRoomName(room); err != nil {
				return err
			}

			f, err := db.open(dirRooms, n)
			if err != nil {
				return err
			}

			r := &Room{f: f}
			db.rooms[room] = r
			r.loop()

			recs, err := csv.NewReader(f).ReadAll()
			if err != nil {
				return err
			}
			for _, rec := range recs {
				m, err := msgFromRec(rec)
				if err != nil {
					return err
				}
				r.msgs = append(r.msgs, m)
			}
		}

		if _, ok := db.rooms[roomWelcome]; !ok {
			if err := db.AddRoom(roomWelcome); err != nil {
				return err
			}
		}

		return nil
	}()

	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func (db *DB) open(path ...string) (*os.File, error) {
	path = append([]string{db.dir}, path...)
	return os.OpenFile(
		filepath.Join(path...),
		os.O_RDWR|os.O_CREATE|os.O_SYNC,
		0644,
	)
}

func (db *DB) Close() error {
	outer := db.users.Close()
	for _, r := range db.rooms {
		if err := r.f.Close(); err != nil {
			outer = err
		}
	}
	return outer
}

func write(w io.Writer, rec []string) error {
	c := csv.NewWriter(w)
	if err := c.Write(rec); err != nil {
		return err
	}
	c.Flush()
	return c.Error()
}
