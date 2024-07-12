package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var directory string

func main() {
	args := os.Args[1:]
	if len(args) >= 2 && args[0] == "--directory" {
		directory = args[1]
	}

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err.Error())
			continue
		}
		go handleConnection(conn)
	}
}

func compressString(s string) ([]byte, error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	_, err := gz.Write([]byte(s))
	if err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	requestLine, _ := reader.ReadString('\n')
	parts := strings.Split(requestLine, " ")
	request_method := parts[0]
	path := parts[1]

	h := make(map[string]string)
	for {
		line, _ := reader.ReadString('\n')
		if line == "\r\n" {
			break
		}

		hp := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(hp) == 2 {
			h[strings.ToLower(hp[0])] = strings.TrimSpace(hp[1])
		}

	}

	var body string
	switch {
	case path == "/":
		body = "HTTP/1.1 200 OK\r\n\r\n"
	case strings.HasPrefix(path, "/echo/"):

		var isCompressed bool
		var compressed []byte

		acceptEncoding := h["accept-encoding"]
		str := strings.TrimPrefix(path, "/echo/")
		body = "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n"
		if strings.Contains(acceptEncoding, "gzip") {
			body += "Content-Encoding: gzip\r\n"
			isCompressed = true
			compressed, _ = compressString(str)
		}
		if isCompressed {
			body += fmt.Sprintf("Content-Length: %d\r\n\r\n", len(compressed))
			body += string(compressed)
		} else {
			body += fmt.Sprintf("Content-Length: %d\r\n\r\n", len(str))
			body += str
		}

	case path == "/user-agent":
		userAgent := h["user-agent"]
		body = "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: " + strconv.Itoa(len(userAgent)) + "\r\n\r\n" + userAgent

	case strings.HasPrefix(path, "/files/"):
		filename := strings.TrimPrefix(path, "/files/")
		filePath := filepath.Join(directory, filename)
		if request_method == "GET" {
			if fileContent, err := os.ReadFile(filePath); err == nil {
				contentLength := len(fileContent)
				body = fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length: %d\r\n\r\n%s", contentLength, fileContent)
			} else {
				body = "HTTP/1.1 404 Not Found\r\n\r\n"
			}
		} else {
			cl, _ := strconv.Atoi(h["content-length"])
			c := make([]byte, cl)
			_, err := io.ReadFull(reader, c)
			if err == nil {
				os.WriteFile(filePath, c, 0644)
			}

			body = "HTTP/1.1 201 Created\r\n\r\n"
		}

	default:
		body = "HTTP/1.1 404 Not Found\r\n\r\n"
	}

	conn.Write([]byte(body))
}
