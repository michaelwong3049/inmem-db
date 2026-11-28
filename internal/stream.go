package db

import (
	"cmp"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type StreamEntry struct {
	id   string
	data map[string]string
}

type Stream struct {
	entries []StreamEntry
}

func containsSeq(entryId string) bool {
	return strings.Contains(entryId, "-") && !strings.HasSuffix(entryId, "*")
}

func (s *Stream) top() *StreamEntry {
	if len(s.entries) == 0 {
		return nil
	}

	return &s.entries[len(s.entries)-1]
}

func (s *Stream) lastId() (int, int) {
	top := s.top()
	if top == nil {
		return 0, 0
	}

	ms, seq, _ := parseId(top.id, 0)
	return ms, seq
}

func (s *Stream) createSeq(ms int) (int, error) {
	top := s.top()
	if top == nil {
		return 0, nil
	}

	topMs, topSeq, _ := strings.Cut(top.id, "-")

	topMsInt, err := strconv.Atoi(topMs)
	if err != nil {
		return 0, err
	}

	if topMsInt != ms {
		return 0, nil
	}

	seq, err := strconv.Atoi(topSeq)
	if err != nil {
		return 0, err
	}

	return seq + 1, nil
}

func createId(entryId string, s *Stream) (string, error) {
	if containsSeq(entryId) {
		return entryId, nil
	}

	// create ms
	var ms int
	if entryId == "*" {
		ms = int(time.Now().UTC().UnixMilli())
	} else {
		var err error
		ms, err = strconv.Atoi(strings.TrimSuffix(entryId, "-*"))
		if err != nil {
			return "", fmt.Errorf("Error parsing ms of entry id %q", entryId)
		}
	}

	// create seq
	seq, err := s.createSeq(ms)
	if err != nil {
		return "", err
	}

	return strconv.Itoa(ms) + "-" + strconv.Itoa(seq), nil
}

func parseId(id string, defaultSeq int) (int, int, error) {
	switch id {
	case "-":
		return 0, 0, nil
	case "+":
		return math.MaxInt, math.MaxInt, nil
	}

	msPart, seqPart, hasSeq := strings.Cut(id, "-")

	ms, err := strconv.Atoi(msPart)
	if err != nil {
		return 0, 0, err
	}

	if !hasSeq {
		return ms, defaultSeq, nil
	}

	seq, err := strconv.Atoi(seqPart)
	if err != nil {
		return 0, 0, err
	}

	return ms, seq, nil
}

func compareId(aMs, aSeq, bMs, bSeq int) int {
	if c := cmp.Compare(aMs, bMs); c != 0 {
		return c
	}

	return cmp.Compare(aSeq, bSeq)
}

func bulkString(s string) string {
	return "$" + strconv.Itoa(len(s)) + "\r\n" + s + "\r\n"
}

func (e StreamEntry) encode() string {
	var b strings.Builder

	b.WriteString("*2\r\n" + bulkString(e.id) + "*" + strconv.Itoa(len(e.data)*2) + "\r\n")
	for field, value := range e.data {
		b.WriteString(bulkString(field) + bulkString(value))
	}

	return b.String()
}

func (s *Stream) append(id string, pairs []string) {
	data := make(map[string]string, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		data[pairs[i]] = pairs[i+1]
	}

	s.entries = append(s.entries, StreamEntry{id: id, data: data})
}

func createStream() *Stream {
	return &Stream{}
}

func (s *Stream) entriesAfter(afterMs, afterSeq int) []StreamEntry {
	for i, entry := range s.entries {
		ms, seq, err := parseId(entry.id, 0)
		if err != nil {
			continue
		}

		if compareId(ms, seq, afterMs, afterSeq) > 0 {
			return s.entries[i:]
		}
	}

	return nil
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

	db.pushed.Broadcast()

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
	blocking := false
	timeoutMs := 0

	if strings.ToUpper(args[0]) == "BLOCK" {
		ms, err := strconv.Atoi(args[1])
		if err != nil {
			return []byte("-ERR timeout is not an integer or out of range\r\n"), nil
		}
		if ms < 0 {
			return []byte("-ERR timeout is negative\r\n"), nil
		}

		blocking, timeoutMs = true, ms
		args = args[2:]
	}

	if strings.ToUpper(args[0]) != "STREAMS" {
		return []byte("-ERR syntax error\r\n"), nil
	}

	rest := args[1:]
	if len(rest) == 0 || len(rest)%2 != 0 {
		return []byte("-ERR Unbalanced 'xread' list of streams: for each stream key an ID, '+', or '$' must be specified.\r\n"), nil
	}

	keys, ids := rest[:len(rest)/2], rest[len(rest)/2:]

	afterMs := make([]int, len(ids))
	afterSeq := make([]int, len(ids))
	for i, id := range ids {
		if id == "$" {
			if stream, ok := db.data[keys[i]].(*Stream); ok {
				afterMs[i], afterSeq[i] = stream.lastId()
			}
			continue
		}

		ms, seq, err := parseId(id, 0)
		if err != nil {
			return []byte("-ERR Invalid stream ID specified as stream command argument\r\n"), nil
		}

		afterMs[i], afterSeq[i] = ms, seq
	}

	var deadline time.Time
	if blocking && timeoutMs > 0 {
		deadline = time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)

		timer := time.AfterFunc(time.Until(deadline), func() {
			db.mu.Lock()
			defer db.mu.Unlock()
			db.pushed.Broadcast()
		})
		defer timer.Stop()
	}

	for {
		if reply := db.readNewer(keys, afterMs, afterSeq); reply != nil {
			return reply, nil
		}

		if !blocking {
			return []byte("*-1\r\n"), nil
		}

		if timeoutMs > 0 && !time.Now().Before(deadline) {
			return []byte("*-1\r\n"), nil
		}

		db.pushed.Wait()
	}
}

func (db *DB) readNewer(keys []string, afterMs, afterSeq []int) []byte {
	var streams strings.Builder
	count := 0

	for i, key := range keys {
		v, ok := db.data[key]
		if !ok {
			continue
		}

		stream, ok := v.(*Stream)
		if !ok {
			return []byte("-WRONGTYPE Operation against a key holding the wrong kind of value\r\n")
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
		return nil
	}

	return []byte("*" + strconv.Itoa(count) + "\r\n" + streams.String())
}
