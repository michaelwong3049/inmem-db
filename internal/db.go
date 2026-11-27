package db

import (
	"fmt"
	"math"
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

func (db *DB) rpush(args []string) ([]byte, error) {
	key, node_vals := args[0], args[1:]

	db.expired(key)

	var ll *doublyLinkedList
	if v, ok := db.data[key]; ok {
		ll, ok = v.(*doublyLinkedList)
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

	db.pushed.Broadcast()

	return []byte(":" + strconv.Itoa(ll.size) + "\r\n"), nil
}

func (db *DB) lpush(args []string) ([]byte, error) {
	if len(args) < 2 {
		return []byte("-ERR wrong number of arguments for 'lpush' command\r\n"), nil
	}

	var ll *doublyLinkedList
	key, node_vals := args[0], args[1:]

	db.expired(key)

	if v, ok := db.data[key]; ok {
		ll, ok = v.(*doublyLinkedList)
		if !ok {
			return []byte("-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"), nil
		}
	} else {
		ll = createList()
		db.data[key] = ll
	}

	for _, n := range node_vals {
		ll.appendleft(n)
	}

	db.pushed.Broadcast()

	return []byte(":" + strconv.Itoa(ll.size) + "\r\n"), nil
}

func (db *DB) lrange(args []string) ([]byte, error) {
	if len(args) != 3 {
		return []byte("-ERR wrong number of arguments for 'lrange' command\r\n"), nil
	}

	key := args[0]
	start, err := strconv.Atoi(args[1])
	if err != nil {
		return []byte("-ERR value is not an integer or out of range\r\n"), fmt.Errorf("Error: value is not an integer or out of range")
	}
	end, err := strconv.Atoi(args[2])
	if err != nil {
		return []byte("-ERR value is not an integer or out of range\r\n"), fmt.Errorf("Error: value is not an integer or out of range")
	}

	db.expired(key)

	v, ok := db.data[key]
	if !ok {
		return []byte("*0\r\n"), nil
	}

	ll, ok := v.(*doublyLinkedList)
	if !ok {
		return []byte("-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"), nil
	}

	return ll.lrangeHelper(start, end), nil
}

func (db *DB) llen(args []string) ([]byte, error) {
	key := args[0]

	v, ok := db.data[key]
	if !ok {
		return []byte(":0\r\n"), nil
	}

	ll, ok := v.(*doublyLinkedList)
	if !ok {
		return []byte("-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"), nil
	}

	return []byte(":" + strconv.Itoa(ll.size) + "\r\n"), nil
}

func (db *DB) lpop(args []string) ([]byte, error) {
	key := args[0]

	v, ok := db.data[key]
	if !ok {
		if len(args) > 1 {
			return []byte("*-1\r\n"), nil
		}
		return []byte("$-1\r\n"), nil
	}

	ll, ok := v.(*doublyLinkedList)
	if !ok {
		return []byte("-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"), nil
	}

	if len(args) == 1 {
		val := ll.popleft()
		return []byte("$" + strconv.Itoa(len(val)) + "\r\n" + val + "\r\n"), nil
	}

	count, err := strconv.Atoi(args[1])
	if err != nil || count < 0 {
		return []byte("-ERR value is out of range, must be positive\r\n"), nil
	}

	count = min(count, ll.size)

	var reply strings.Builder
	reply.WriteString("*" + strconv.Itoa(count) + "\r\n")

	for range count {
		val := ll.popleft()
		reply.WriteString("$" + strconv.Itoa(len(val)) + "\r\n" + val + "\r\n")
	}

	return []byte(reply.String()), nil
}

func (db *DB) blpop(args []string) ([]byte, error) {
	key := args[0]

	timeout, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return []byte("-ERR timeout is not a float or out of range\r\n"), nil
	}
	if timeout < 0 {
		return []byte("-ERR timeout is negative\r\n"), nil
	}

	// set function such that when the expired time arrives, we release this blpop
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(time.Duration(timeout * float64(time.Second)))

		timer := time.AfterFunc(time.Until(deadline), func() {
			db.mu.Lock()
			defer db.mu.Unlock()
			db.pushed.Broadcast()
		})
		defer timer.Stop()
	}

	for {
		if v, ok := db.data[key]; ok {
			ll, ok := v.(*doublyLinkedList)
			if !ok {
				return []byte("-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"), nil
			}

			val := ll.popleft()

			return []byte("*2\r\n" +
				"$" + strconv.Itoa(len(key)) + "\r\n" + key + "\r\n" +
				"$" + strconv.Itoa(len(val)) + "\r\n" + val + "\r\n"), nil
		}

		if timeout > 0 && !time.Now().Before(deadline) {
			return []byte("*-1\r\n"), nil
		}

		db.pushed.Wait()
	}
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

func (db *DB) xadd(args []string) ([]byte, error) {
	streamKey, entryId, pairs := args[0], args[1], args[2:]

	if len(pairs)%2 != 0 {
		return []byte("-ERR didn't not provide a value for every key\r\n"), nil
	}

	var stream *Stream
	if v, ok := db.data[streamKey]; ok {
		stream, ok = v.(*Stream)
		if !ok {
			return []byte("-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"), nil
		}
	} else {
		stream = createStream()
	}

	id, err := createId(entryId, stream)
	if err != nil {
		return []byte("-ERR Invalid stream ID specified as stream command argument\r\n"), nil
	}

	stream.append(id, pairs)
	db.data[streamKey] = stream

	return []byte("$" + strconv.Itoa(len(id)) + "\r\n" + id + "\r\n"), nil
}

func (db *DB) xrange(args []string) ([]byte, error) {
	streamKey, start, end := args[0], args[1], args[2]

	v, ok := db.data[streamKey]
	if !ok {
		return []byte("*0\r\n"), nil
	}

	stream, ok := v.(*Stream)
	if !ok {
		return []byte("-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"), nil
	}

	startMs, startSeq, err := parseId(start, 0)
	if err != nil {
		return []byte("-ERR Invalid stream ID specified as stream command argument\r\n"), nil
	}

	endMs, endSeq, err := parseId(end, math.MaxInt)
	if err != nil {
		return []byte("-ERR Invalid stream ID specified as stream command argument\r\n"), nil
	}

	var entries strings.Builder
	count := 0

	for _, entry := range stream.entries {
		ms, seq, err := parseId(entry.id, 0)
		if err != nil {
			continue
		}

		if compareId(ms, seq, startMs, startSeq) < 0 {
			continue
		}

		if compareId(ms, seq, endMs, endSeq) > 0 {
			break
		}

		count++
		entries.WriteString("*2\r\n" + bulkString(entry.id) + "*" + strconv.Itoa(len(entry.data)*2) + "\r\n")
		for field, value := range entry.data {
			entries.WriteString(bulkString(field) + bulkString(value))
		}
	}

	return []byte("*" + strconv.Itoa(count) + "\r\n" + entries.String()), nil
}

func (db *DB) xread(args []string) ([]byte, error) {
	if strings.ToUpper(args[0]) != "STREAMS" {
		return []byte("-ERR syntax error\r\n"), nil
	}

	rest := args[1:]
	if len(rest)%2 != 0 {
		return []byte("-ERR Unbalanced 'xread' list of streams: for each stream key an ID, '+', or '$' must be specified.\r\n"), nil
	}

	keys, ids := rest[:len(rest)/2], rest[len(rest)/2:]

	afterMs := make([]int, len(ids))
	afterSeq := make([]int, len(ids))
	for i, id := range ids {
		ms, seq, err := parseId(id, 0)
		if err != nil {
			return []byte("-ERR Invalid stream ID specified as stream command argument\r\n"), nil
		}

		afterMs[i], afterSeq[i] = ms, seq
	}

	var streams strings.Builder
	count := 0

	for i, key := range keys {
		v, ok := db.data[key]
		if !ok {
			continue
		}

		stream, ok := v.(*Stream)
		if !ok {
			return []byte("-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"), nil
		}

		newer := stream.entriesAfter(afterMs[i], afterSeq[i])
		if len(newer) == 0 {
			continue
		}

		count++
		streams.WriteString("*2\r\n" + bulkString(key) + "*" + strconv.Itoa(len(newer)) + "\r\n")
		for _, entry := range newer {
			streams.WriteString(entry.encode())
		}
	}

	if count == 0 {
		return []byte("*-1\r\n"), nil
	}

	return []byte("*" + strconv.Itoa(count) + "\r\n" + streams.String()), nil
}
