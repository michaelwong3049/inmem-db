package db

// test line from Claude: checking that edits show up in Neovim
import (
	"testing"
	"time"
)

func TestPing(t *testing.T) {
	db := CreateDB()

	cmd := []string{"PING"}
	result, err := db.ExecuteCommand(cmd)

	if err != nil {
		t.Errorf("error executing cmd: %v", err)
	}

	expected := "+PONG\r\n"
	if string(result) != expected {
		t.Errorf("got %q, expected %q", result, expected)
	}
}

func TestSet(t *testing.T) {
	db := CreateDB()

	tests := []struct {
		key      string
		val      string
		expected string
	}{
		{"name", "joe", "+OK\r\n"},
		{"city", "nyc", "+OK\r\n"},
		{"lang", "go", "+OK\r\n"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result, err := db.ExecuteCommand([]string{"SET", tt.key, tt.val})
			if err != nil {
				t.Fatalf("error executing SET: %v", err)
			}
			if string(result) != tt.expected {
				t.Errorf("SET got %q, expected %q", result, tt.expected)
			}

			// the key should now exist in the db with the value we set
			got, ok := db.data[tt.key]
			if !ok {
				t.Fatalf("expected key %q to exist in db", tt.key)
			}
			if got != tt.val {
				t.Errorf("db[%q] = %v, expected %q", tt.key, got, tt.val)
			}
		})
	}

	// a key that was never SET should not exist
	t.Run("missing key", func(t *testing.T) {
		if _, ok := db.data["missing"]; ok {
			t.Errorf("expected key %q to not exist in db", "missing")
		}

		result, err := db.ExecuteCommand([]string{"GET", "missing"})
		if err != nil {
			t.Fatalf("error executing GET: %v", err)
		}

		expected := "$-1\r\n"
		if string(result) != expected {
			t.Errorf("GET got %q, expected %q", result, expected)
		}
	})
}

// run executes cmd against db and fails the test if it returns an error.
func run(t *testing.T, db *DB, cmd ...string) string {
	t.Helper()

	result, err := db.ExecuteCommand(cmd)
	if err != nil {
		t.Fatalf("error executing %v: %v", cmd, err)
	}

	return string(result)
}

func TestLPush(t *testing.T) {
	t.Run("creates list and returns length", func(t *testing.T) {
		db := CreateDB()

		if got := run(t, db, "LPUSH", "list", "A"); got != ":1\r\n" {
			t.Errorf("LPUSH got %q, expected %q", got, ":1\r\n")
		}
		if got := run(t, db, "LPUSH", "list", "B", "C"); got != ":3\r\n" {
			t.Errorf("LPUSH got %q, expected %q", got, ":3\r\n")
		}
	})

	t.Run("each value is pushed onto the head", func(t *testing.T) {
		db := CreateDB()
		run(t, db, "LPUSH", "list", "A", "B", "C")

		expected := "*3\r\n$1\r\nC\r\n$1\r\nB\r\n$1\r\nA\r\n"
		if got := run(t, db, "LRANGE", "list", "0", "-1"); got != expected {
			t.Errorf("LRANGE got %q, expected %q", got, expected)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		db := CreateDB()
		run(t, db, "SET", "str", "val")

		expected := "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"
		if got := run(t, db, "LPUSH", "str", "A"); got != expected {
			t.Errorf("LPUSH got %q, expected %q", got, expected)
		}
	})

	t.Run("expired key is replaced by a new list", func(t *testing.T) {
		db := CreateDB()
		run(t, db, "SET", "key", "val", "PX", "0")

		if got := run(t, db, "LPUSH", "key", "A"); got != ":1\r\n" {
			t.Errorf("LPUSH got %q, expected %q", got, ":1\r\n")
		}
		if _, ok := db.expiredTimes["key"]; ok {
			t.Errorf("expected old TTL for %q to be removed", "key")
		}
	})
}

func TestRPush(t *testing.T) {
	t.Run("appends to the tail and returns length", func(t *testing.T) {
		db := CreateDB()

		if got := run(t, db, "RPUSH", "list", "A", "B", "C"); got != ":3\r\n" {
			t.Errorf("RPUSH got %q, expected %q", got, ":3\r\n")
		}

		expected := "*3\r\n$1\r\nA\r\n$1\r\nB\r\n$1\r\nC\r\n"
		if got := run(t, db, "LRANGE", "list", "0", "-1"); got != expected {
			t.Errorf("LRANGE got %q, expected %q", got, expected)
		}
	})

	t.Run("expired key is replaced by a new list", func(t *testing.T) {
		db := CreateDB()
		run(t, db, "SET", "key", "val", "PX", "0")

		if got := run(t, db, "RPUSH", "key", "A"); got != ":1\r\n" {
			t.Errorf("RPUSH got %q, expected %q", got, ":1\r\n")
		}
	})
}

func TestLRange(t *testing.T) {
	db := CreateDB()
	run(t, db, "RPUSH", "list", "A", "B", "C")

	tests := []struct {
		name     string
		start    string
		end      string
		expected string
	}{
		{"whole list", "0", "-1", "*3\r\n$1\r\nA\r\n$1\r\nB\r\n$1\r\nC\r\n"},
		{"single element", "0", "0", "*1\r\n$1\r\nA\r\n"},
		{"middle to end", "1", "2", "*2\r\n$1\r\nB\r\n$1\r\nC\r\n"},
		{"negative indexes", "-2", "-1", "*2\r\n$1\r\nB\r\n$1\r\nC\r\n"},
		{"end past the tail", "0", "100", "*3\r\n$1\r\nA\r\n$1\r\nB\r\n$1\r\nC\r\n"},
		{"start before the head", "-100", "1", "*2\r\n$1\r\nA\r\n$1\r\nB\r\n"},
		{"start after end", "2", "1", "*0\r\n"},
		{"start past the tail", "5", "10", "*0\r\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, db, "LRANGE", "list", tt.start, tt.end); got != tt.expected {
				t.Errorf("LRANGE %s %s got %q, expected %q", tt.start, tt.end, got, tt.expected)
			}
		})
	}

	t.Run("missing key", func(t *testing.T) {
		if got := run(t, db, "LRANGE", "missing", "0", "-1"); got != "*0\r\n" {
			t.Errorf("LRANGE got %q, expected %q", got, "*0\r\n")
		}
	})

	t.Run("expired key", func(t *testing.T) {
		run(t, db, "SET", "key", "val", "PX", "0")

		if got := run(t, db, "LRANGE", "key", "0", "-1"); got != "*0\r\n" {
			t.Errorf("LRANGE got %q, expected %q", got, "*0\r\n")
		}
	})
}

func TestLLen(t *testing.T) {
	db := CreateDB()
	run(t, db, "RPUSH", "list", "A", "B", "C", "D")

	if got := run(t, db, "LLEN", "list"); got != ":4\r\n" {
		t.Errorf("LLEN got %q, expected %q", got, ":4\r\n")
	}

	if got := run(t, db, "LLEN", "missing"); got != ":0\r\n" {
		t.Errorf("LLEN got %q, expected %q", got, ":0\r\n")
	}
}

func TestLPop(t *testing.T) {
	db := CreateDB()
	run(t, db, "RPUSH", "list_key", "A", "B", "C", "D")

	if got := run(t, db, "LPOP", "list_key"); got != "$1\r\nA\r\n" {
		t.Errorf("LPOP got %q, expected %q", got, "$1\r\nA\r\n")
	}

	if got := run(t, db, "LPOP", "missing"); got != "$-1\r\n" {
		t.Errorf("LPOP got %q, expected %q", got, "$-1\r\n")
	}
}

func TestLPopCount(t *testing.T) {
	t.Run("pops count elements from the head", func(t *testing.T) {
		db := CreateDB()
		run(t, db, "RPUSH", "list_key", "A", "B", "C", "D")

		expected := "*2\r\n$1\r\nA\r\n$1\r\nB\r\n"
		if got := run(t, db, "LPOP", "list_key", "2"); got != expected {
			t.Errorf("LPOP got %q, expected %q", got, expected)
		}
	})

	t.Run("count of zero", func(t *testing.T) {
		db := CreateDB()
		run(t, db, "RPUSH", "list_key", "A")

		if got := run(t, db, "LPOP", "list_key", "0"); got != "*0\r\n" {
			t.Errorf("LPOP got %q, expected %q", got, "*0\r\n")
		}
	})

	t.Run("missing key", func(t *testing.T) {
		db := CreateDB()

		if got := run(t, db, "LPOP", "missing", "2"); got != "*-1\r\n" {
			t.Errorf("LPOP got %q, expected %q", got, "*-1\r\n")
		}
	})

	t.Run("invalid count", func(t *testing.T) {
		db := CreateDB()
		run(t, db, "RPUSH", "list_key", "A")

		expected := "-ERR value is out of range, must be positive\r\n"
		for _, c := range []string{"-1", "abc"} {
			if got := run(t, db, "LPOP", "list_key", c); got != expected {
				t.Errorf("LPOP %s got %q, expected %q", c, got, expected)
			}
		}
	})
}

func TestBLPop(t *testing.T) {
	t.Run("pops immediately when key exists", func(t *testing.T) {
		db := CreateDB()
		run(t, db, "RPUSH", "list_key", "A", "B")

		expected := "*2\r\n$8\r\nlist_key\r\n$1\r\nA\r\n"
		if got := run(t, db, "BLPOP", "list_key", "0"); got != expected {
			t.Errorf("BLPOP got %q, expected %q", got, expected)
		}
	})

	t.Run("blocks until another client pushes", func(t *testing.T) {
		db := CreateDB()

		done := make(chan string)
		go func() {
			result, _ := db.ExecuteCommand([]string{"BLPOP", "list_key", "0"})
			done <- string(result)
		}()

		// BLPOP should still be waiting, since nothing has been pushed
		select {
		case got := <-done:
			t.Fatalf("BLPOP returned %q before anything was pushed", got)
		case <-time.After(50 * time.Millisecond):
		}

		// this only gets the lock if the waiting BLPOP released it
		run(t, db, "RPUSH", "list_key", "A")

		expected := "*2\r\n$8\r\nlist_key\r\n$1\r\nA\r\n"
		select {
		case got := <-done:
			if got != expected {
				t.Errorf("BLPOP got %q, expected %q", got, expected)
			}
		case <-time.After(time.Second):
			t.Fatal("BLPOP did not wake up after RPUSH")
		}
	})

	t.Run("times out with a null array", func(t *testing.T) {
		db := CreateDB()

		start := time.Now()
		if got := run(t, db, "BLPOP", "list_key", "0.1"); got != "*-1\r\n" {
			t.Errorf("BLPOP got %q, expected %q", got, "*-1\r\n")
		}
		if waited := time.Since(start); waited < 100*time.Millisecond {
			t.Errorf("BLPOP returned after %v, expected to wait at least 100ms", waited)
		}
	})

	t.Run("invalid timeout", func(t *testing.T) {
		db := CreateDB()

		if got := run(t, db, "BLPOP", "list_key", "abc"); got != "-ERR timeout is not a float or out of range\r\n" {
			t.Errorf("BLPOP got %q", got)
		}
		if got := run(t, db, "BLPOP", "list_key", "-1"); got != "-ERR timeout is negative\r\n" {
			t.Errorf("BLPOP got %q", got)
		}
	})
}

func TestType(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		db := CreateDB()
		run(t, db, "SET", "str_key", "hello")

		if got := run(t, db, "TYPE", "str_key"); got != "+string\r\n" {
			t.Errorf("TYPE got %q, expected %q", got, "+string\r\n")
		}
	})

	t.Run("list", func(t *testing.T) {
		db := CreateDB()
		run(t, db, "RPUSH", "list_key", "A")

		if got := run(t, db, "TYPE", "list_key"); got != "+list\r\n" {
			t.Errorf("TYPE got %q, expected %q", got, "+list\r\n")
		}
	})

	t.Run("missing key", func(t *testing.T) {
		db := CreateDB()

		if got := run(t, db, "TYPE", "missing"); got != "+none\r\n" {
			t.Errorf("TYPE got %q, expected %q", got, "+none\r\n")
		}
	})

	t.Run("expired key", func(t *testing.T) {
		db := CreateDB()
		run(t, db, "SET", "str_key", "hello")
		db.expiredTimes["str_key"] = time.Now().Add(-time.Second)

		if got := run(t, db, "TYPE", "str_key"); got != "+none\r\n" {
			t.Errorf("TYPE got %q, expected %q", got, "+none\r\n")
		}
	})

	t.Run("too few arguments", func(t *testing.T) {
		db := CreateDB()

		if _, err := db.ExecuteCommand([]string{"TYPE"}); err == nil {
			t.Error("expected an error for TYPE with no key")
		}
	})
}
