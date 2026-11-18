package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

/*
open tcp port --> any connection that comes in route their request with a goroutine --> goroutine should take this connection parse what you write and see if it pertains
	to the given methods that we support --> run that function and return back to the user

1. create the server and allow tcp connections to connect to this server
2. create the cli that users
*/

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
		fmt.Println("Error getting line in readInteger")
		return 0, 0, err
	}

	intValue, err := strconv.Atoi(strings.TrimSuffix(line[1:], "\r\n"))
	fmt.Println("intValue: ", intValue)
	if err != nil {
		fmt.Println("Error getting integer value in readInteger: ", err)
		return 0, 0, err
	}

	return line[0], intValue, nil
}

func readParameter(reader *bufio.Reader, symbol byte, length int) (string, error) {
	param := make([]byte, length+2)

	_, err := io.ReadFull(reader, param)
	if err != nil {
		fmt.Println("Error reading bytes in readParamter")
		os.Exit(1)
	}

	return string(param[:length]), nil
}

func parseCommand(reader *bufio.Reader) ([]string, error) {
	symbol, arraySize, err := readInteger(reader)
	if err != nil {
		fmt.Println("Error reading first integer in parseCommand")
		os.Exit(1)
	}

	if symbol != ARRAY {
		fmt.Println("Did not receive ARRAY type as the first byte")
		os.Exit(1)
	}

	cmd := []string{}
	for range arraySize {
		symbol, length, err := readInteger(reader) // since we are using go, we can just read the parameters now

		if err != nil {
			fmt.Println("Error parsing integer: iteration")
			os.Exit(1)
		}

		param, err := readParameter(reader, symbol, length)
		if err != nil {
			fmt.Println("Error parsing parameter: iteration")
			os.Exit(1)
		}
		cmd = append(cmd, param)
		fmt.Println(cmd)
	}

	return cmd, nil
}

func handleConnection(conn net.Conn) {
	// use the bufio and scanner in order to take the bytes that are coming in from the client
	// parse that information
	// run the required functions in order to update for our key value store

	defer conn.Close()

	reader := bufio.NewReader(conn)
	cmd, err := parseCommand(reader)
	if err != nil {
		fmt.Println("Error parsing command")
	}

	// now we can "run" this command
	fmt.Println(cmd)
}

func main() {
	listener, err := net.Listen("tcp", ":6379")
	if err != nil {
		log.Fatal("Failed to open TCP connection/server")
	}

	defer listener.Close()

	fmt.Println("Started server")
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal("Failed to accept connection")
		}
		go handleConnection(conn)
	}
}
