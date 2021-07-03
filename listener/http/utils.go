package http

import (
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"strings"
)

// RemoveHopByHopHeaders remove hop-by-hop header
func RemoveHopByHopHeaders(header http.Header) {
	// Strip hop-by-hop header based on RFC:
	// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html#sec13.5.1
	// https://www.mnot.net/blog/2011/07/11/what_proxies_must_do

	header.Del("Proxy-Connection")
	header.Del("Proxy-Authenticate")
	header.Del("Proxy-Authorization")
	header.Del("TE")
	header.Del("Trailers")
	header.Del("Transfer-Encoding")
	header.Del("Upgrade")

	connections := header.Get("Connection")
	header.Del("Connection")
	if len(connections) == 0 {
		return
	}
	for _, h := range strings.Split(connections, ",") {
		header.Del(strings.TrimSpace(h))
	}
}

// RemoveExtraHTTPHostPort remove extra host port (example.com:80 --> example.com)
// It resolves the behavior of some HTTP servers that do not handle host:80 (e.g. baidu.com)
func RemoveExtraHTTPHostPort(req *http.Request) {
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}

	if pHost, port, err := net.SplitHostPort(host); err == nil && port == "80" {
		host = pHost
	}

	req.Host = host
	req.URL.Host = host
}

// ParseBasicProxyAuthorization parse header Proxy-Authorization and return base64-encoded credential
func ParseBasicProxyAuthorization(request *http.Request) string {
	value := request.Header.Get("Proxy-Authorization")
	if !strings.HasPrefix(value, "Basic ") {
		return ""
	}

	return value[6:] // value[len("Basic "):]
}

// DecodeBasicProxyAuthorization decode base64-encoded credential
func DecodeBasicProxyAuthorization(credential string) (string, string, error) {
	plain, err := base64.StdEncoding.DecodeString(credential)
	if err != nil {
		return "", "", err
	}

	login := strings.Split(string(plain), ":")
	if len(login) != 2 {
		return "", "", errors.New("invalid login")
	}

	return login[0], login[1], nil
}
