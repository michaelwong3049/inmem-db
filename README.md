**inmem-db** is a small in-memory key-value database written in Go. It speaks the Redis wire protocol (RESP) over TCP, so you can talk to it with `redis-cli` or any Redis client library.

## Features

- RESP parsing over TCP, with one goroutine per client connection
- A shared in-memory store guarded by a mutex
- String values with optional expiry
- Lists backed by a doubly linked list

## Supported commands

| Command | Description |
| --- | --- |
| `PING` | Returns `PONG` |
| `ECHO message` | Returns `message` |
| `SET key value [EX\|PX ttl]` | Stores a string, optionally with an expiry |
| `GET key` | Returns the string at `key`, or nil if it is missing or expired |
| `RPUSH key value [value ...]` | Appends one or more values to a list and returns its new length |
| `LRANGE key start stop` | Returns list elements from `start` to `stop` (inclusive, negative indexes count from the end) |

## Running

```sh
go run .
```

The server listens on port `6379`. In another terminal:

```sh
redis-cli PING
redis-cli SET greeting hello
redis-cli GET greeting
redis-cli RPUSH numbers 1 2 3
redis-cli LRANGE numbers 0 -1
```
