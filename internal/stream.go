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
