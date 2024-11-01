# How tunnel works?

## CONNECT HTTP Method

The CONNECT HTTP method requests that a proxy establish a HTTP tunnel to a destination server, and if successful, blindly forward data in both directions until the tunnel is closed.

The request target is unique to this method in that it consists of only the host and port number of the tunnel destination, separated by a colon (see Syntax for details). Any 2XX successful response status code means that the proxy will switch to 'tunnel mode' and any data in the success response body is from the server identified by the request target.

If a website is behind a proxy and it's enforced via network rules that all external traffic must pass through the proxy, the CONNECT method allows you to establish a TLS (HTTPS) connection with that website:

The client asks the proxy to tunnel the TCP connection to the desired destination.
The proxy server makes a secure connection to the server on behalf of the client.
Once the connection is established, the proxy server continues to relay the TCP stream to and from the client.
Aside from enabling secure access to websites behind proxies, a HTTP tunnel provides a way to allow traffic that would otherwise be restricted (SSH or FTP) over the HTTP(S) protocol.

CONNECT is a hop-by-hop method, meaning proxies will only forward the CONNECT request if there is another inbound proxy in front of the origin server since most origin servers do not implement CONNECT.

## Our Goals

- create http proxy server
- block connect to advertizing domains
- just proxy traffic from / to sources / destinations

## Steps to our goals

- create listener
- creating accept loop in a separate goroutine allow us to serve multiple connections simultaneously
- in accept loop we have to do a http.ReadRequest, to read http request metadata, like schema, host, etc...
- we processing request method, if it's CONNECT, we have to start handle tunneling: we response with HTTP/1.1 200 Connection Established to the client and and start process data transferring request data from client to destination and server response from destination to clinet in separate goroutines, otherwise we just handle http requests
- if we are dealing with CONNECT method, we can't make any html responses back to the client, or something like that, we cat just close(BREAK) connection. If we need to look inside https traffic, and change responses, it is known as MITM or Man Nn The Middle attack...
- handle tunneling: we make a tcp connection to target ip address, we can use here our provider dns server or we can use cloudflare, google dns, etc... and then we just proxying data from and to the client
- in case of http request handle we just do a transport.RoundTrip(req), getting result Body and writing response back to the client
- that's it :-)
