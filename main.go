package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
)

const (
	STRING  = '+'
	ERROR   = '-'
	INTEGER = ':'
	BULK    = '$'
	ARRAY   = '*'
)

func readInteger(reader *bufio.Reader) (byte, int, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return 0, 0, err
	}

	intValue, err := strconv.Atoi(strings.TrimSuffix(line[1:], "\r\n"))
	if err != nil {
		return 0, 0, err
	}

	return line[0], intValue, nil
}

func readParameter(reader *bufio.Reader, symbol byte, length int) (string, error) {
	param := make([]byte, length+2)

	_, err := io.ReadFull(reader, param)
	if err != nil {
		return "", err
	}

	return string(param[:length]), nil
}

func parseCommand(reader *bufio.Reader) ([]string, error) {
	cmd := []string{}

	symbol, arraySize, err := readInteger(reader)
	if err != nil {
		return cmd, err
	}

	if symbol != ARRAY {
		return cmd, fmt.Errorf("Error: first symbol is not symbol")
	}

	for range arraySize {
		symbol, length, err := readInteger(reader)

		if err != nil {
			return cmd, err
		}

		param, err := readParameter(reader, symbol, length)
		if err != nil {
			return cmd, err
		}
		cmd = append(cmd, param)
	}

	return cmd, nil
}

func handleConnection(conn net.Conn, db *DB) {
	// use the bufio and scanner in order to take the bytes that are coming in from the client
	// parse that information
	// run the required functions in order to update for our key value store

	defer conn.Close()

	reader := bufio.NewReader(conn)
	for {
		cmd, err := parseCommand(reader)
		if err != nil {
			fmt.Println("Error parsing command, ", err)
			return
		}

		// now we can "run" this command
		reply, err := db.executeCommand(cmd)
		if err != nil {
			fmt.Println("Error handling this command: ", err)
			return
		}

		conn.Write(reply)
	}
}

func main() {
	listener, err := net.Listen("tcp", ":6379")
	if err != nil {
		log.Fatal("Failed to open TCP connection/server")
	}

	defer listener.Close()
	db := createDB()

	fmt.Println("Started server")
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal("Failed to accept connection")
		}
		go handleConnection(conn, db)
	}
}
