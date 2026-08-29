// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/log"

	"go.uber.org/zap"
)

// Bounds on what an unauthenticated peer can cost this process. A listener the
// network can reach answers 403 and 401 to strangers, and those refusals must
// themselves be cheap.
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second
	maxHeaderBytes    = 64 << 10

	// maxLoggedHeaderLen caps how much of a rejected header reaches the log.
	// The value is attacker-controlled on exactly the requests this server
	// refuses, so logging it whole lets a stranger write to the operator's disk
	// for free.
	maxLoggedHeaderLen = 128
)

// isLoopbackHostname reports whether host names a loopback interface. host may
// carry brackets ("[::1]") but not a port.
func isLoopbackHostname(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" {
		return true
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return addr.IsLoopback()
}

// splitHostPort separates a "host" or "host:port" authority. It returns the
// bare host (no brackets, lower-cased), the port ("" when absent), and whether
// the value parsed at all.
//
// A value whose port is not a plain number does not parse. net.SplitHostPort
// splits on the last colon without looking at what follows it, so
// "127.0.0.1:8080@example.com" would otherwise reduce to a loopback host.
func splitHostPort(authority string) (host, port string, ok bool) {
	authority = strings.TrimSpace(strings.ToLower(authority))
	if authority == "" {
		return "", "", false
	}
	if h, p, err := net.SplitHostPort(authority); err == nil {
		if !isNumericPort(p) {
			return "", "", false
		}
		return strings.Trim(h, "[]"), p, true
	}
	// No port. A bracketed literal is an IPv6 address; an unbracketed value
	// containing a colon is either a bare IPv6 address or malformed.
	if strings.HasPrefix(authority, "[") && strings.HasSuffix(authority, "]") {
		return strings.Trim(authority, "[]"), "", true
	}
	if strings.Contains(authority, ":") {
		// A bare IPv6 address is legitimate in an allow-list entry even though
		// it is not legal in a Host header. Accept it only if it really parses
		// as one.
		if _, err := netip.ParseAddr(authority); err == nil {
			return authority, "", true
		}
		return "", "", false
	}
	return authority, "", true
}

// isNumericPort reports whether port is a non-empty run of decimal digits.
func isNumericPort(port string) bool {
	if port == "" {
		return false
	}
	for _, r := range port {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// addrIsLoopbackOnly reports whether every address this listener can be reached
// on is a loopback address.
//
// It reads the address actually bound, never the configured string: a
// configured value can resolve differently than expected. The unspecified
// address ("::" or "0.0.0.0", which is what a bare ":port" produces) is
// reachable from the network and is therefore not loopback-only.
//
// This decides only whether an allow-list is REQUIRED and whether loopback
// names are implicitly acceptable. It deliberately decides nothing about
// credentials — see forgejo.SetRequireRequestToken for why.
func addrIsLoopbackOnly(a net.Addr) bool {
	tcpAddr, ok := a.(*net.TCPAddr)
	if !ok {
		// Unknown address shape: assume it is reachable. Failing towards the
		// stricter policy is the safe direction.
		return false
	}
	addr, ok := netip.AddrFromSlice(tcpAddr.IP)
	if !ok {
		return false
	}
	return addr.IsLoopback()
}

// hostPolicy decides which Host values this listener answers to.
type hostPolicy struct {
	// allowed holds the operator's declared names, lower-cased. An entry may
	// carry a port ("mcp.example.org:8443"), in which case it must match
	// exactly, or omit one, in which case any port matches — the port already
	// had to be ours for the request to arrive.
	allowed []string
	// loopbackOnly means the listener cannot be reached from the network, so
	// loopback names are acceptable whatever else was declared. Declaring a
	// name for a proxy must not stop the operator's own machine from
	// connecting directly.
	loopbackOnly bool
}

func (p hostPolicy) permits(value string) bool {
	host, port, ok := splitHostPort(value)
	if !ok {
		return false
	}
	for _, entry := range p.allowed {
		eHost, ePort, eOK := splitHostPort(entry)
		if !eOK || eHost != host {
			continue
		}
		if ePort == "" || ePort == port {
			return true
		}
	}
	return p.loopbackOnly && isLoopbackHostname(host)
}

// originPolicy decides which Origin values this listener accepts.
//
// Host and Origin are deliberately NOT checked against the same list. They mean
// different things: a Host names this server, an Origin names the web page
// making the request. Reusing one list for both silently accepts every other
// service sharing a declared hostname on any port, and every plaintext-HTTP
// origin on it — which is the cross-site case the Origin check exists to stop.
type originPolicy struct {
	// allowed holds full origins, normalised by normalizeOrigin. Empty means no
	// browser origin is accepted, which is correct for a server whose clients
	// are not browsers.
	allowed []string
}

func (p originPolicy) permits(value string) bool {
	normalised, ok := normalizeOrigin(value)
	if !ok {
		return false
	}
	for _, entry := range p.allowed {
		if entry == normalised {
			return true
		}
	}
	return false
}

// normalizeOrigin reduces an origin to "scheme://host[:port]", dropping the
// port when it is the default for the scheme so that "https://x" and
// "https://x:443" compare equal. Anything that is not a hierarchical http or
// https URL — "null", an opaque value, a bare hostname, a file URL — does not
// normalise and is therefore never permitted.
func normalizeOrigin(value string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	if parsed.User != nil {
		// Userinfo has no meaning in an origin and is a classic way to make a
		// hostile authority read as a friendly one.
		return "", false
	}
	host, port, ok := splitHostPort(parsed.Host)
	if !ok || host == "" {
		return "", false
	}
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	if port == "" {
		return scheme + "://" + host, true
	}
	return scheme + "://" + host + ":" + port, true
}

// truncateForLog bounds an attacker-controlled value before it reaches the log.
func truncateForLog(v string) string {
	if len(v) <= maxLoggedHeaderLen {
		return v
	}
	return v[:maxLoggedHeaderLen] + "…(truncated)"
}

// guardRequests is the middleware required by the Model Context Protocol's
// Streamable HTTP transport: "Servers MUST validate the Origin header on all
// incoming connections to prevent DNS rebinding attacks", alongside "Servers
// SHOULD implement proper authentication for all connections".
//
// It applies three checks, in order of how cheaply they refuse:
//
//   - Host, against DNS rebinding. An attacker's page rebinds its own name to
//     127.0.0.1; the browser then treats the local server as same-origin and no
//     cross-origin check runs. The Host header still carries the attacker's
//     name, which is what catches it.
//   - Origin, against ordinary cross-site request forgery. A page at any origin
//     can address a loopback listener directly with a correct Host header. Only
//     the Origin header distinguishes that from a real client, and the library
//     underneath does not look at it. A request with no Origin header at all is
//     allowed: a non-browser client — which is what an MCP client normally is —
//     sends none, and the Host check still applies to it.
//   - Authorization, when this server does not fall back to its own credential.
//     Refusing at the door rather than at the forge client means an anonymous
//     caller cannot open a session, enumerate the tool catalogue, or hold an
//     event stream open either.
func guardRequests(next http.Handler, hosts hostPolicy, origins originPolicy, requireAuth bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hosts.permits(r.Host) {
			log.Warn("Rejected request with unacceptable Host header",
				log.StringField("host", truncateForLog(r.Host)),
				log.StringField("path", truncateForLog(r.URL.Path)),
			)
			http.Error(w, "Forbidden: unacceptable Host header", http.StatusForbidden)
			return
		}
		// Read the header map directly: a present-but-empty Origin must not be
		// mistaken for an absent one, and Header.Get cannot tell them apart.
		if origin, present := r.Header["Origin"]; present {
			value := ""
			if len(origin) > 0 {
				value = origin[0]
			}
			if !origins.permits(value) {
				log.Warn("Rejected request with unacceptable Origin header",
					log.StringField("origin", truncateForLog(value)),
					log.StringField("path", truncateForLog(r.URL.Path)),
				)
				http.Error(w, "Forbidden: unacceptable Origin header", http.StatusForbidden)
				return
			}
		}
		if requireAuth && extractToken(r.Header.Get("Authorization")) == "" {
			log.Warn("Rejected request with no usable Authorization header",
				log.StringField("path", truncateForLog(r.URL.Path)),
			)
			w.Header().Set("WWW-Authenticate", `Bearer realm="forgejo-mcp"`)
			http.Error(w, "Unauthorized: this server requires a per-request token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// transportConfig is the decision this change exists to make explicit, resolved
// once from configuration before anything is bound.
type transportConfig struct {
	// listenHosts are the addresses to bind. A loopback request resolves to
	// both loopback families, because binding only 127.0.0.1 leaves a client
	// that resolves "localhost" to ::1 unable to connect — and connecting to
	// "localhost" is what this project's own documentation tells clients to do.
	listenHosts []string
	// loopbackOnly is the configured intent; the bound listeners are checked
	// against it afterwards rather than trusted from here.
	loopbackOnly bool
	hosts        hostPolicy
	origins      originPolicy
	// requireAuth is true unless the operator explicitly opted back in to the
	// old behaviour. It is never inferred from the bind address.
	requireAuth bool
}

// resolveTransportConfig validates the configuration and refuses before
// anything is bound. Binding first and validating afterwards would briefly open
// a public socket on a misconfigured start.
func resolveTransportConfig(transportName string) (transportConfig, error) {
	cfg := transportConfig{requireAuth: !flag.AllowOperatorTokenFallback}

	host := strings.TrimSpace(flag.Host)
	if host == "" {
		return cfg, fmt.Errorf(
			"refusing to start: %s transport has an empty bind address; "+
				"pass -host with an address to listen on", transportName)
	}

	cfg.loopbackOnly = isLoopbackHostname(host)
	if cfg.loopbackOnly {
		cfg.listenHosts = []string{"127.0.0.1", "::1"}
	} else {
		cfg.listenHosts = []string{host}
	}

	cfg.hosts = hostPolicy{allowed: normalizeList(flag.AllowedHosts), loopbackOnly: cfg.loopbackOnly}
	if !cfg.loopbackOnly && len(cfg.hosts.allowed) == 0 {
		return cfg, fmt.Errorf(
			"refusing to start: %s transport is configured to bind %s, which the network can reach, "+
				"but no allowed hosts are declared; pass -allowed-hosts (or FORGEJO_MCP_ALLOWED_HOSTS) "+
				"with the host names clients will use, or bind to loopback instead",
			transportName, host)
	}

	for _, raw := range normalizeList(flag.AllowedOrigins) {
		normalised, ok := normalizeOrigin(raw)
		if !ok {
			return cfg, fmt.Errorf(
				"refusing to start: %q is not a usable origin for -allowed-origins; "+
					"an origin is a scheme, a host and an optional port, for example https://console.example.org",
				raw)
		}
		cfg.origins.allowed = append(cfg.origins.allowed, normalised)
	}

	return cfg, nil
}

// normalizeList lower-cases entries and drops empty ones.
func normalizeList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.ToLower(strings.TrimSpace(v))
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// newMCPHTTPServer wraps handler in the policy stack for an already-resolved
// configuration.
func newMCPHTTPServer(handler http.Handler, cfg transportConfig) *http.Server {
	return &http.Server{
		Handler:           guardRequests(handler, cfg.hosts, cfg.origins, cfg.requireAuth),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		// Go replaces srv.Handler entirely for "OPTIONS *" unless this is set,
		// which would route that one request shape around the whole policy.
		DisableGeneralOptionsHandler: true,
	}
}

// serveMCPOverHTTP binds the listeners for a network transport and serves
// handler on all of them.
//
// Both network transports go through here rather than calling the library's own
// Start method, so that the listen address, the header checks and the
// credential policy are decided in exactly one place. A transport that bound
// its own listener would silently miss all three.
func serveMCPOverHTTP(transportName string, handler http.Handler, port int) error {
	cfg, err := resolveTransportConfig(transportName)
	if err != nil {
		return err
	}

	forgejo.SetRequireRequestToken(cfg.requireAuth)

	var listeners []net.Listener
	closeAll := func() {
		for _, ln := range listeners {
			_ = ln.Close()
		}
	}
	for _, host := range cfg.listenHosts {
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			if cfg.loopbackOnly && len(listeners) > 0 {
				// One loopback family is enough; a machine with IPv6 disabled
				// must still start.
				log.Debug("Loopback family unavailable, continuing",
					log.StringField("address", addr),
					log.ErrorField(err),
				)
				continue
			}
			closeAll()
			log.Error("Failed to bind listener",
				log.StringField("transport", transportName),
				log.StringField("address", addr),
				log.ErrorField(err),
			)
			return fmt.Errorf("failed to bind %s listener on %s: %w", transportName, addr, err)
		}
		if cfg.loopbackOnly && !addrIsLoopbackOnly(ln.Addr()) {
			// The configuration said loopback and the kernel disagreed. Trust
			// the listener, not the string.
			closeAll()
			_ = ln.Close()
			return fmt.Errorf(
				"refusing to start: %s transport asked for loopback but bound %s",
				transportName, ln.Addr())
		}
		listeners = append(listeners, ln)
	}
	if len(listeners) == 0 {
		return fmt.Errorf("failed to bind any %s listener", transportName)
	}
	defer closeAll()

	bound := make([]string, 0, len(listeners))
	for _, ln := range listeners {
		bound = append(bound, ln.Addr().String())
	}
	logListening(transportName, bound, cfg)

	srv := newMCPHTTPServer(handler, cfg)
	return serveAll(srv, listeners)
}

// serveAll runs srv on every listener and returns the first error.
func serveAll(srv *http.Server, listeners []net.Listener) error {
	var (
		wg    sync.WaitGroup
		once  sync.Once
		first error
	)
	for _, ln := range listeners {
		wg.Add(1)
		go func(ln net.Listener) {
			defer wg.Done()
			if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				once.Do(func() { first = err; _ = srv.Close() })
			}
		}(ln)
	}
	wg.Wait()
	return first
}

func logListening(transportName string, bound []string, cfg transportConfig) {
	fields := []zap.Field{
		log.StringField("transport", transportName),
		log.StringField("address", strings.Join(bound, ", ")),
	}
	if cfg.requireAuth {
		fields = append(fields, log.StringField("authentication", "every request must carry its own Authorization header"))
	} else {
		fields = append(fields, log.StringField("authentication",
			"DISABLED: requests with no Authorization header are served using this server's own credential"))
	}
	if len(cfg.origins.allowed) > 0 {
		fields = append(fields, log.StringField("allowed_origins", strings.Join(cfg.origins.allowed, ",")))
	}
	if cfg.loopbackOnly {
		fields = append(fields,
			log.StringField("reachable_from", "this machine only, unless a proxy forwards to it"),
			log.StringField("to_expose", "set -host to an address the network can reach, and declare -allowed-hosts"),
		)
		if cfg.requireAuth {
			log.Info("MCP server listening on loopback only", fields...)
		} else {
			log.Warn("MCP server listening on loopback only, with credential fallback enabled", fields...)
		}
		return
	}
	fields = append(fields, log.StringField("allowed_hosts", strings.Join(cfg.hosts.allowed, ",")))
	log.Warn("MCP server listening on a network-reachable address", fields...)
}
