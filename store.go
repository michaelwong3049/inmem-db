package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DB struct {
	mu           sync.Mutex
	data         map[string]any
	expiredTimes map[string]time.Time
}

type handler func(db *DB, cmd []string) ([]byte, error)

type command struct {
	fn          handler
	minArgCount int
}

var commands = map[string]command{
	"SET":   {fn: (*DB).set, minArgCount: 2},
	"GET":   {fn: (*DB).get, minArgCount: 1},
	"RPUSH": {fn: (*DB).rpush, minArgCount: 2},
	"PING":  {fn: (*DB).ping, minArgCount: 0},
	"ECHO":  {fn: (*DB).echo, minArgCount: 1},
}

func createDB() *DB {
	return &DB{
		data:         make(map[string]any),
		expiredTimes: make(map[string]time.Time),
	}
}

func (db *DB) executeCommand(cmd []string) ([]byte, error) {
	op, args := strings.ToUpper(cmd[0]), cmd[1:]

	c, ok := commands[op]
	if !ok {
		return []byte{}, fmt.Errorf("ERROR: invalid operation: %q", op)
	}

	if len(args) < c.minArgCount {
		return []byte{}, fmt.Errorf("Too few arguments, need atleast %d", c.minArgCount)
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	return c.fn(db, args)
}

func (db *DB) rpush(args []string) ([]byte, error) {
	key, node_vals := args[0], args[1:]

	var ll *DoublyLinkedList
	if v, ok := db.data[key]; ok {
		ll, ok = v.(*DoublyLinkedList)
		if !ok {
			return []byte("-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"), fmt.Errorf("Error running rpush")
		}
	} else {
		ll = createList()
		db.data[key] = ll
	}

	for _, val := range node_vals {
		ll.append(val)
	}

	return []byte(":" + strconv.Itoa(ll.size) + "\r\n"), nil
}

// expired reports whether key's TTL has passed (now >= expiry), and if so
// deletes the key. Callers must hold db.mu.
func (db *DB) expired(key string) bool {
	expiry, ok := db.expiredTimes[key]
	if !ok || time.Now().Before(expiry) {
		return false
	}

	delete(db.data, key)
	delete(db.expiredTimes, key)
	return true
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

func (db *DB) echo(args []string) ([]byte, error) {
	msg := args[0]

	return []byte("$" + strconv.Itoa(len(msg)) + "\r\n" + msg + "\r\n"), nil
}

func (db *DB) ping(_ []string) ([]byte, error) {
	return []byte("+PONG\r\n"), nil
}
