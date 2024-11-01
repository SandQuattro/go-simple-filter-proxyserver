package main

import (
	"bufio"
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"simple-proxy-server/internal/proxy"
	zerologger "simple-proxy-server/pkg/logger"
	"strings"
	"syscall"

	"github.com/rs/zerolog"
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
	zerologger.InitLogger(true)
	logger := zerologger.GetCtxLogger(context.Background())
	err := run(logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Error running the server")
	}
}

func run(logger zerolog.Logger) error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	err := proxy.SetOsProxy(8080)
	if err != nil {
		logger.Error().Err(err).Msg("error set system-wide proxy")
		return err
	}

	logger.Info().Msg("system-wide proxy was set successfully")

	// Listen for incoming connections on port 8080
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to listen on port 8080")
	}
	logger.Info().Msg("Proxy server listening on port 8080")

	go func() {
		for {
			select {
			case <-ctx.Done():
				// останавливаем горутину
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					logger.Println("Failed to accept connection:", err)
					continue
				}

				// process our new connections concurrently
				go handleLoop(ctx, conn)
			}
		}
	}()

	stopped := make(chan struct{})
	go func() {
		// Используем буферизированный канал, как рекомендовано внутри signal.Notify функции
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		// Блокируемся и ожидаем из канала quit - interrupt signal,
		// чтобы сделать gracefully shutdown с таймаутом в 10 сек
		<-quit

		// Завершаем работу горутин
		cancelFunc()

		err := proxy.UnsetOsProxy()
		if err != nil {
			logger.Error().Err(err).Msg("error unset system-wide proxy")
		}
		logger.Info().Msg("system-wide proxy was unset successfully")

		close(stopped)
	}()

	<-stopped

	logger.Info().Msg("Server gracefully stopped, bye, bye!")

	return nil
}

func handleLoop(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	logger := zerologger.GetCtxLogger(ctx)

	reader := bufio.NewReader(conn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		logger.Printf("Error reading request: %v", err)
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

	if processBlockedHosts(ctx, conn, req) {
		return
	}

	// this is a request to start proxy tunnel
	if req.Method == "CONNECT" {
		// Handle HTTPS tunneling
		logger.Printf("Establishing tunnel to %s", host)
		conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		handleTunneling(ctx, conn, host)
	} else {
		// Handle regular HTTP requests
		logger.Printf("Handling HTTP request for %s", req.URL)
		handleHTTP(ctx, conn, req)
	}
}

func handleTunneling(ctx context.Context, clientConn net.Conn, destHost string) {
	defer clientConn.Close()

	logger := zerologger.GetCtxLogger(ctx)

	remoteConn, err := net.Dial("tcp", destHost)
	if err != nil {
		logger.Printf("Failed to connect to host %s: %v", destHost, err)
		return
	}
	defer remoteConn.Close()

	// Start copying data between client and remote host
	go func() {
		io.Copy(remoteConn, clientConn)
	}()
	io.Copy(clientConn, remoteConn)
}

func handleHTTP(ctx context.Context, clientConn net.Conn, req *http.Request) {
	logger := zerologger.GetCtxLogger(ctx)

	logger.Printf("handling request, scheme:%s\n", req.URL.Scheme)

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
		logger.Printf("Error forwarding request: %v, err:%v", req, err)
		return
	}
	defer resp.Body.Close()

	// Write the response back to the client
	err = resp.Write(clientConn)
	if err != nil {
		logger.Printf("Error writing response to client: %v", err)
	}
}

func processBlockedHosts(ctx context.Context, conn net.Conn, req *http.Request) bool {
	logger := zerologger.GetCtxLogger(ctx)

	// blocking advertising domains
	for _, domain := range domainList {
		if strings.Contains(req.Host, domain) {
			logger.Printf("Blocking access to %s", req.Host)

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
