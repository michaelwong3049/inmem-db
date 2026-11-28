package db

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DB struct {
	mu           sync.Mutex
	pushed       *sync.Cond
	data         map[string]any
	expiredTimes map[string]time.Time
}

type handler func(db *DB, cmd []string) ([]byte, error)

type command struct {
	fn          handler
	minArgCount int
}

var commands = map[string]command{
	"PING":   {fn: (*DB).ping, minArgCount: 0},
	"ECHO":   {fn: (*DB).echo, minArgCount: 1},
	"SET":    {fn: (*DB).set, minArgCount: 2},
	"GET":    {fn: (*DB).get, minArgCount: 1},
	"LPUSH":  {fn: (*DB).lpush, minArgCount: 2},
	"RPUSH":  {fn: (*DB).rpush, minArgCount: 2},
	"LRANGE": {fn: (*DB).lrange, minArgCount: 3},
	"LLEN":   {fn: (*DB).llen, minArgCount: 1},
	"LPOP":   {fn: (*DB).lpop, minArgCount: 1},
	"BLPOP":  {fn: (*DB).blpop, minArgCount: 2},
	"TYPE":   {fn: (*DB).keyType, minArgCount: 1},
	"XADD":   {fn: (*DB).xadd, minArgCount: 4},
	"XRANGE": {fn: (*DB).xrange, minArgCount: 3},
	"XREAD":  {fn: (*DB).xread, minArgCount: 3},
}

func CreateDB() *DB {
	db := &DB{
		data:         make(map[string]any),
		expiredTimes: make(map[string]time.Time),
	}
	db.pushed = sync.NewCond(&db.mu)

	return db
}

func (db *DB) ExecuteCommand(cmd []string) ([]byte, error) {
	op, args := strings.ToUpper(cmd[0]), cmd[1:]

	c, ok := commands[op]
	if !ok {
		return []byte{}, fmt.Errorf("-ERROR: invalid operation: %q", op)
	}

	if len(args) < c.minArgCount {
		return []byte{}, fmt.Errorf("Too few arguments, need atleast %d", c.minArgCount)
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	return c.fn(db, args)
}

func (db *DB) expired(key string) bool {
	expiry, ok := db.expiredTimes[key]
	if !ok || time.Now().Before(expiry) {
		return false
	}

	delete(db.data, key)
	delete(db.expiredTimes, key)
	return true
}

func (db *DB) ping(_ []string) ([]byte, error) {
	return []byte("+PONG\r\n"), nil
}

func (db *DB) echo(args []string) ([]byte, error) {
	msg := args[0]

	return []byte("$" + strconv.Itoa(len(msg)) + "\r\n" + msg + "\r\n"), nil
}

func (db *DB) set(args []string) ([]byte, error) {
	key, val := args[0], args[1]

	db.data[key] = val

	if len(args) > 2 {
		ttl_unit, ttl_value := args[2], args[3]
		ttl_value_int, err := strconv.Atoi(ttl_value)

		if err != nil {
			return []byte{}, fmt.Errorf("Error parsing ttl_value %q for key %q", ttl_value, key)
		}

		switch strings.ToUpper(ttl_unit) {
		case "EX":
			db.expiredTimes[key] = time.Now().Add(time.Duration(ttl_value_int) * time.Millisecond)
		case "PX":
			db.expiredTimes[key] = time.Now().Add(time.Duration(ttl_value_int) * time.Second)
		}
	}

	return []byte("+OK\r\n"), nil
}

func (db *DB) get(args []string) ([]byte, error) {
	key := args[0]

	if db.expired(key) {
		return []byte("$-1\r\n"), nil
	}

	v, ok := db.data[key]
	if !ok {
		return []byte("$-1\r\n"), nil
	}

	s, ok := v.(string)
	if !ok {
		return []byte("$-1\r\n"), fmt.Errorf("Error: value is not of string type, cannot GET")
	}

	return []byte("$" + strconv.Itoa(len(s)) + "\r\n" + s + "\r\n"), nil
}

func (db *DB) keyType(args []string) ([]byte, error) {
	key := args[0]

	db.expired(key)

	v, ok := db.data[key]
	if !ok {
		return []byte("+none\r\n"), nil
	}

	switch v.(type) {
	case string:
		return []byte("+string\r\n"), nil
	case *doublyLinkedList:
		return []byte("+list\r\n"), nil
	case *Stream:
		return []byte("+stream\r\n"), nil
	default:
		return []byte("-ERR unknown value type\r\n"), nil
	}
}
