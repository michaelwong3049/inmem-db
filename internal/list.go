package db

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type node struct {
	val  string
	next *node
	prev *node
}

type doublyLinkedList struct {
	head *node
	tail *node
	data map[string]string
	size int
}

func (ll *doublyLinkedList) appendleft(val string) {
	head, first := ll.head, ll.head.next
	n := &node{val: val, prev: head, next: first}

	head.next = n
	first.prev = n
	ll.size++
}

func (ll *doublyLinkedList) append(val string) {
	last := ll.tail.prev
	n := &node{val: val, prev: last, next: ll.tail}

	last.next = n
	ll.tail.prev = n
	ll.size++
}

func (ll *doublyLinkedList) popleft() string {
	first := ll.head.next
	newNext := first.next

	newNext.prev = ll.head
	ll.head.next = newNext

	return first.val
}

func (ll *doublyLinkedList) lrangeHelper(start, end int) []byte {
	if start < 0 {
		start = ll.size + start
	}

	if end < 0 {
		end = ll.size + end
	}

	if start < 0 {
		start = 0
	}

	if end >= ll.size {
		end = ll.size - 1
	}

	if start > end {
		return []byte("*0\r\n")
	}

	node := ll.head.next
	for range start {
		node = node.next
	}

	var reply strings.Builder
	reply.WriteString("*" + strconv.Itoa(end-start+1) + "\r\n")

	for i := start; i <= end; i++ {
		reply.WriteString("$" + strconv.Itoa(len(node.val)) + "\r\n" + node.val + "\r\n")
		node = node.next
	}

	fmt.Println(reply.String())
	return []byte(reply.String())
}

func createList() *doublyLinkedList {
	head := &node{val: "head"}
	tail := &node{val: "tail"}

	head.next = tail
	tail.prev = head

	return &doublyLinkedList{
		head: head,
		tail: tail,
		data: make(map[string]string),
	}
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
