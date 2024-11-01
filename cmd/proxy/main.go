package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

var domainList []string

func init() {
	file, err := os.Open("blocked_domains.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		domain := strings.TrimSpace(scanner.Text())
		if domain != "" {
			domainList = append(domainList, domain)
		}
	}

	if err = scanner.Err(); err != nil {
		panic(err)
	}
}

func main() {
	// Listen for incoming connections on port 8080
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalln("Failed to listen on port 8080:", err)
	}
	log.Println("Proxy server listening on port 8080")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Failed to accept connection:", err)
			continue
		}

		// process our new connections concurrently
		go handleLoop(conn)
	}
}

func handleLoop(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		log.Printf("Error reading request: %v", err)
		return
	}

	// Determine the destination host and port
	host := req.Host
	if !strings.Contains(host, ":") {
		if req.Method == "CONNECT" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	if processBlockedHosts(conn, req) {
		return
	}

	// this is a request to start proxy tunnel
	if req.Method == "CONNECT" {
		// Handle HTTPS tunneling
		log.Printf("Establishing tunnel to %s", host)
		conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		handleTunneling(conn, host)
	} else {
		// Handle regular HTTP requests
		log.Printf("Handling HTTP request for %s", req.URL)
		handleHTTP(conn, req)
	}
}

func handleTunneling(clientConn net.Conn, destHost string) {
	defer clientConn.Close()
	remoteConn, err := net.Dial("tcp", destHost)
	if err != nil {
		log.Printf("Failed to connect to host %s: %v", destHost, err)
		return
	}
	defer remoteConn.Close()

	// Start copying data between client and remote host
	go func() {
		io.Copy(remoteConn, clientConn)
	}()
	io.Copy(clientConn, remoteConn)
}

func handleHTTP(clientConn net.Conn, req *http.Request) {
	log.Printf("handling request, scheme:%s\n", req.URL.Scheme)

	transport := &http.Transport{}
	defer clientConn.Close()

	// Modify the Request URI since http.Client doesn't include the host in the request URI
	req.RequestURI = ""

	// Remove proxy-related headers to prevent loops
	req.Header.Del("Proxy-Connection")
	req.Header.Del("Proxy-Authenticate")
	req.Header.Del("Proxy-Authorization")
	req.Header.Del("Connection")

	// Forward the request to the actual remote host
	resp, err := transport.RoundTrip(req)
	if err != nil {
		log.Printf("Error forwarding request: %v, err:%v", req, err)
		return
	}
	defer resp.Body.Close()

	// Write the response back to the client
	err = resp.Write(clientConn)
	if err != nil {
		log.Printf("Error writing response to client: %v", err)
	}
}

func processBlockedHosts(conn net.Conn, req *http.Request) bool {
	// blocking advertising domains
	for _, domain := range domainList {
		if strings.Contains(req.Host, domain) {
			log.Printf("Blocking access to %s", req.Host)

			if req.Method == "CONNECT" {
				// Respond with 403 Forbidden for HTTPS sites
				response := "HTTP/1.1 403 Forbidden\r\n" +
					"Connection: close\r\n" +
					"\r\n"
				conn.Write([]byte(response))
			} else {
				// Serve custom HTML page for HTTP sites
				response := "HTTP/1.1 200 OK\r\n" +
					"Content-Type: text/html\r\n" +
					"Connection: close\r\n" +
					"\r\n" +
					"<html><body><h1>Access Denied</h1><p>Blocked</p></body></html>"
				conn.Write([]byte(response))
			}

			return true
		}
	}

	return false
}
