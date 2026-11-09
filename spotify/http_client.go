package spotify

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

func newSpotifyHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = spotifyProxyForRequest
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

func spotifyProxyForRequest(req *http.Request) (*url.URL, error) {
	if spotifyEnvironmentProxyConfigured() {
		proxy, err := http.ProxyFromEnvironment(req)
		if proxy != nil || err != nil {
			return proxy, err
		}
		if spotifyProxyBypassed(
			req.URL.Hostname(),
			os.Getenv("NO_PROXY"),
			os.Getenv("no_proxy"),
		) {
			return nil, nil
		}
		if allProxy := firstSpotifyNonEmpty(
			os.Getenv("ALL_PROXY"),
			os.Getenv("all_proxy"),
		); allProxy != "" {
			return spotifyParseProxyURL(allProxy), nil
		}
		return nil, nil
	}
	if strings.TrimSpace(os.Getenv("NO_PROXY")) != "" ||
		strings.TrimSpace(os.Getenv("no_proxy")) != "" {
		return nil, nil
	}
	return spotifySystemProxy(req)
}

func spotifyEnvironmentProxyConfigured() bool {
	for _, name := range []string{
		"HTTP_PROXY",
		"http_proxy",
		"HTTPS_PROXY",
		"https_proxy",
		"ALL_PROXY",
		"all_proxy",
	} {
		if strings.TrimSpace(os.Getenv(name)) != "" {
			return true
		}
	}
	return false
}

func spotifyParseProxyURL(raw string) *url.URL {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || strings.TrimSpace(parsed.Host) == "" {
		return nil
	}
	return parsed
}

func spotifyProxyBypassed(host string, lists ...string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		return false
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil && ip.IsLoopback() {
		return true
	}

	for _, list := range lists {
		for _, rawPattern := range strings.FieldsFunc(
			list,
			func(value rune) bool {
				return value == ',' || value == ';'
			},
		) {
			pattern := strings.TrimSpace(strings.ToLower(rawPattern))
			if pattern == "" {
				continue
			}
			if pattern == "<local>" {
				if !strings.Contains(host, ".") {
					return true
				}
				continue
			}
			if pattern == "*" {
				return true
			}
			if strings.Contains(pattern, "://") {
				if parsed, err := url.Parse(pattern); err == nil {
					pattern = parsed.Hostname()
				}
			}
			pattern = strings.TrimPrefix(pattern, ".")
			if strings.Contains(pattern, "/") {
				pattern = strings.SplitN(pattern, "/", 2)[0]
			}
			if matched, err := path.Match(pattern, host); err == nil && matched {
				return true
			}
			if host == pattern || strings.HasSuffix(host, "."+pattern) {
				return true
			}
		}
	}
	return false
}
