package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// This function serializes into the RESP format that Redis uses
func serializeInput(input string) []byte {
	splitInput := strings.Split(input, " ")
	byteInput := []byte{}

	// add the first array symbol, length of the command, and CRLF
	byteInput = append(byteInput, '*')
	byteInput = append(byteInput, strconv.Itoa(len(splitInput))...)
	byteInput = append(byteInput, "\r\n"...)

	for _, s := range splitInput {
		fmt.Println(s, len(s))
		// append the string length symbol and string length value itself
		byteInput = append(byteInput, '$')
		byteInput = append(byteInput, strconv.Itoa(len(s))...)
		byteInput = append(byteInput, "\r\n"...)

		// append the current string in bytes + CRLF clause
		byteInput = append(byteInput, s...)
		byteInput = append(byteInput, "\r\n"...)
	}

	return byteInput
}

func main() {
	fmt.Println("Starting client...")
	conn, err := net.Dial("tcp", "localhost:6379")
	if err != nil {
		fmt.Println("could not connect to tcp server")
		os.Exit(1)
	}

	defer conn.Close()
	stdin := bufio.NewScanner(os.Stdin)
	server := bufio.NewReader(conn)

	for {
		fmt.Print("redis> ")
		if stdin.Scan() {
			input := stdin.Text()
			conn.Write(serializeInput(input))
		}
	}
}
