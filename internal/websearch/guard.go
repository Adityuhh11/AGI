package websearch

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// A page the model asks for is chosen by a language model that a web page may be talking to. The address is checked on the
// connection itself, after DNS and for every redirect, so neither a hostname that resolves to 127.0.0.1 nor a redirect to one
// gets through. The chat model's own server, the search engine and anything else on localhost stay out of reach.

var errLocal = errors.New("that address is on this computer or the local network")

// carrierNAT is 100.64.0.0/10, which netip does not count as private.
var carrierNAT = netip.MustParsePrefix("100.64.0.0/10")

func refuseLocal(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("%w: %s", errLocal, host)
	}
	ip = ip.Unmap()
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() ||
		ip.IsUnspecified() || carrierNAT.Contains(ip) {
		return fmt.Errorf("%w: %s", errLocal, ip)
	}
	return nil
}

// guardedClient is an HTTP client for the open web. allowLocal exists for tests, whose servers are on 127.0.0.1.
func guardedClient(allowLocal bool) *http.Client {
	dialer := &net.Dialer{Timeout: 8 * time.Second}
	if !allowLocal {
		dialer.Control = refuseLocal
	}
	return &http.Client{
		Timeout: requestLimit,
		Transport: &http.Transport{
			// no Proxy: a proxy would be dialled instead of the site, and the check would be on the proxy's address
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   8 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			MaxIdleConns:          4,
			IdleConnTimeout:       30 * time.Second,
		},
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 4 {
				return errors.New("the page redirects too many times")
			}
			return nil
		},
	}
}
