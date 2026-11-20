package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

type DB struct {
	mu   sync.Mutex
	data map[string]any
}

type handler func(db *DB, cmd []string) ([]byte, error)

type command struct {
	fn       handler
	argCount int
}

var commands = map[string]command{
	"SET":  {fn: (*DB).set, argCount: 2},
	"GET":  {fn: (*DB).get, argCount: 1},
	"PING": {fn: (*DB).ping, argCount: 0},
	"ECHO": {fn: (*DB).echo, argCount: 1},
}

func createDB() *DB {
	return &DB{
		data: make(map[string]any),
	}
}

func (db *DB) executeCommand(cmd []string) ([]byte, error) {
	op, args := strings.ToUpper(cmd[0]), cmd[1:]

	c, ok := commands[op]
	if !ok {
		return []byte{}, fmt.Errorf("ERROR: invalid operation: %q", op)
	}

	if len(args) != c.argCount {
		return []byte{}, fmt.Errorf("Too many or few arguments, only need %d", c.argCount)
	}

	return c.fn(db, args)
}

func (db *DB) get(args []string) ([]byte, error) {
	key := args[0]
	v, ok := db.data[key]
	if !ok {
		return []byte("$-1\r\n"), fmt.Errorf("Error: key {%q} does not exist", key)
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

	return []byte("+OK\r\n"), nil
}

func (db *DB) echo(args []string) ([]byte, error) {
	msg := args[0]

	return []byte("$" + strconv.Itoa(len(msg)) + "\r\n" + msg + "\r\n"), nil
}

func (db *DB) ping(_ []string) ([]byte, error) {
	return []byte("+PONG\r\n"), nil
}
