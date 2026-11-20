package main

type Node struct {
	val  string
	next *Node
	prev *Node
}

type DoublyLinkedList struct {
	head *Node
	tail *Node
	data map[string]string
	size int
}

// append inserts val just before the tail sentinel.
func (ll *DoublyLinkedList) append(val string) {
	last := ll.tail.prev
	node := &Node{val: val, prev: last, next: ll.tail}

	last.next = node
	ll.tail.prev = node
	ll.size++
}

func createList() *DoublyLinkedList {
	head := &Node{val: "head"}
	tail := &Node{val: "tail"}

	head.next = tail
	tail.prev = head

	return &DoublyLinkedList{
		head: head,
		tail: tail,
		data: make(map[string]string),
	}
}
