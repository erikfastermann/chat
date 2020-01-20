package db

import (
	"fmt"
	"os"
	"sync"
)

type users struct {
	sync.RWMutex
	*os.File
	names  map[string]*user
	tokens map[string]*user
}

type user struct {
	username string
	passHash []byte
	token    string
}

const (
	userUsername = iota
	userPassHash
	userLen
)

func userFromRec(r []string) (*user, error) {
	if len(r) != userLen {
		return nil, fmt.Errorf(
			"user csv record: expected item count %d, got %d",
			userLen,
			len(r),
		)
	}

	return &user{
		username: r[userUsername],
		passHash: []byte(r[userPassHash]),
	}, nil
}

func (u *user) toRec() []string {
	r := make([]string, userLen)
	r[userUsername] = u.username
	r[userPassHash] = string(u.passHash)
	return r
}

func (db *DB) User(username string) (passHash []byte, err error) {
	db.users.RLock()
	defer db.users.RUnlock()

	u, ok := db.users.names[username]
	if !ok {
		return nil, fmt.Errorf("username %q: %w", username, ErrNotFound)
	}
	return u.passHash, nil
}

func (db *DB) UserByToken(token string) (username string, err error) {
	db.users.RLock()
	defer db.users.RUnlock()

	u, ok := db.users.tokens[token]
	if !ok {
		return "", fmt.Errorf("token %q: %w", username, ErrNotFound)
	}
	return u.username, nil
}

func (db *DB) AddUser(username string, passHash []byte) error {
	db.users.Lock()
	defer db.users.Unlock()

	if _, ok := db.users.names[username]; ok {
		return fmt.Errorf("username %q: %w", username, ErrExists)
	}

	u := &user{
		username: username,
		passHash: passHash,
	}
	if err := write(db.users.File, u.toRec()); err != nil {
		return err
	}
	db.users.names[u.username] = u

	return nil
}

func (db *DB) SetToken(username, token string) error {
	db.users.Lock()
	defer db.users.Unlock()

	u, ok := db.users.names[username]
	if !ok {
		return fmt.Errorf("username %q: %w", username, ErrNotFound)
	}
	if u.token != "" {
		delete(db.users.tokens, u.token)
	}

	if token != "" {
		u.token = token
		db.users.tokens[token] = u
	}

	return nil
}
