package db

import (
	"fmt"
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
