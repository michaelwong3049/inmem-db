package db

import (
	"strconv"
	"strings"
	"fmt"
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

// appendleft inserts val just after the head sentinel
func (ll *doublyLinkedList) appendleft(val string) {
	head, first := ll.head, ll.head.next
	n := &node{val: val, prev: head, next: first}

	head.next = n
	first.prev = n
	ll.size++
}

// append inserts val just before the tail sentinel
func (ll *doublyLinkedList) append(val string) {
	last := ll.tail.prev
	n := &node{val: val, prev: last, next: ll.tail}

	last.next = n
	ll.tail.prev = n
	ll.size++
}

// popleft removes the node just after the head sentinel and returns its value.
func (ll *doublyLinkedList) popleft() string {
	first := ll.head.next
	newNext := first.next

	newNext.prev = ll.head
	ll.head.next = newNext

	return first.val
}

// lrangeHelper returns the RESP array reply for the elements from start to stop,
// both inclusive. Negative indexes count from the end (-1 is the last element).
func (ll *doublyLinkedList) lrangeHelper(start, end int) []byte {
	// convert negative indexes to positions counted from the front
	if start < 0 {
		start = ll.size + start
	}

	if end < 0 {
		end = ll.size + end
	}

	// clamp to the list's bounds
	if start < 0 {
		start = 0
	}

	if end >= ll.size {
		end = ll.size - 1
	}

	// return empty array if the range is empty
	if start > end {
		return []byte("*0\r\n")
	}

	// walk forward to the starting node
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
