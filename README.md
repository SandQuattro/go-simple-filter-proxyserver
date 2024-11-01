# How tunnel works?

The CONNECT HTTP method requests that a proxy establish a HTTP tunnel to a destination server, and if successful, blindly forward data in both directions until the tunnel is closed.

The request target is unique to this method in that it consists of only the host and port number of the tunnel destination, separated by a colon (see Syntax for details). Any 2XX successful response status code means that the proxy will switch to 'tunnel mode' and any data in the success response body is from the server identified by the request target.

If a website is behind a proxy and it's enforced via network rules that all external traffic must pass through the proxy, the CONNECT method allows you to establish a TLS (HTTPS) connection with that website:

The client asks the proxy to tunnel the TCP connection to the desired destination.
The proxy server makes a secure connection to the server on behalf of the client.
Once the connection is established, the proxy server continues to relay the TCP stream to and from the client.
Aside from enabling secure access to websites behind proxies, a HTTP tunnel provides a way to allow traffic that would otherwise be restricted (SSH or FTP) over the HTTP(S) protocol.

CONNECT is a hop-by-hop method, meaning proxies will only forward the CONNECT request if there is another inbound proxy in front of the origin server since most origin servers do not implement CONNECT.
