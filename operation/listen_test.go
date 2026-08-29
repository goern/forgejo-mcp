// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/forgejo"

	"github.com/mark3labs/mcp-go/server"
)

// restoreFlags snapshots and restores the package-level configuration these
// tests mutate, so ordering between tests cannot leak policy.
func restoreFlags(t *testing.T) {
	t.Helper()
	host, hosts, origins := flag.Host, flag.AllowedHosts, flag.AllowedOrigins
	fallback, require := flag.AllowOperatorTokenFallback, forgejo.RequireRequestToken()
	flag.Host, flag.AllowedHosts, flag.AllowedOrigins = "127.0.0.1", nil, nil
	flag.AllowOperatorTokenFallback = false
	t.Cleanup(func() {
		flag.Host, flag.AllowedHosts, flag.AllowedOrigins = host, hosts, origins
		flag.AllowOperatorTokenFallback = fallback
		forgejo.SetRequireRequestToken(require)
	})
}

func mustListenLoopback(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on loopback: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return ln
}

// transportCase names one network transport and how to build its handler.
// Every conformance case runs against both, so a transport that stops going
// through the shared listener is a test failure rather than a silent gap.
type transportCase struct {
	name    string
	handler func() http.Handler
	path    string
}

func transports() []transportCase {
	newMCP := func() *server.MCPServer { return server.NewMCPServer("test", "0.0.0") }
	return []transportCase{
		{"sse", func() http.Handler { return newSSEHandler(newMCP()) }, "/sse"},
		{"http", func() http.Handler { return newStreamableHTTPHandler(newMCP()) }, streamableHTTPEndpointPath},
	}
}

// startGuarded runs the transport behind the real policy stack on a loopback
// listener and returns the address to dial.
func startGuarded(t *testing.T, tr transportCase) string {
	t.Helper()
	cfg, err := resolveTransportConfig(tr.name)
	if err != nil {
		t.Fatalf("resolveTransportConfig: %v", err)
	}
	forgejo.SetRequireRequestToken(cfg.requireAuth)
	ln := mustListenLoopback(t)
	srv := newMCPHTTPServer(tr.handler(), cfg)
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return ln.Addr().String()
}

// rawRequest writes the request by hand, because the point of these tests is to
// send headers an HTTP client would never let us send.
func rawRequest(t *testing.T, dialAddr, requestLine string, headers ...string) string {
	t.Helper()
	conn, err := net.DialTimeout("tcp", dialAddr, 3*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", dialAddr, err)
	}
	defer func() { _ = conn.Close() }()

	var b strings.Builder
	b.WriteString(requestLine + "\r\n")
	for _, h := range headers {
		b.WriteString(h + "\r\n")
	}
	b.WriteString("Connection: close\r\n\r\n")

	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write([]byte(b.String())); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 512)
	n, _ := conn.Read(buf)
	line := string(buf[:n])
	if i := strings.Index(line, "\r\n"); i >= 0 {
		return line[:i]
	}
	return line
}

func get(t *testing.T, addr, path string, headers ...string) string {
	t.Helper()
	return rawRequest(t, addr, "GET "+path+" HTTP/1.1", headers...)
}

// --- Host validation ---

func TestForgedHostHeaderIsRejected(t *testing.T) {
	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			restoreFlags(t)
			addr := startGuarded(t, tr)
			// The DNS-rebinding shape: the attacker's own name, resolved to
			// 127.0.0.1, so the connection is loopback but the Host is not.
			if got := get(t, addr, tr.path, "Host: attacker.example.com"); !strings.Contains(got, "403") {
				t.Fatalf("forged Host was not rejected: %q", got)
			}
		})
	}
}

func TestLoopbackHostIsAccepted(t *testing.T) {
	// The control that must succeed. Without it, a guard that rejects
	// everything would pass the test above.
	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			restoreFlags(t)
			addr := startGuarded(t, tr)
			if got := get(t, addr, tr.path, "Host: "+addr, "Authorization: token decoy"); strings.Contains(got, "403") {
				t.Fatalf("legitimate loopback Host was rejected: %q", got)
			}
		})
	}
}

func TestDeclaredHostsDoNotLockOutLoopback(t *testing.T) {
	// Declaring a name for a proxy must not stop the operator's own machine
	// from connecting directly. Making the declared list total would silently
	// break local clients.
	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			restoreFlags(t)
			flag.AllowedHosts = []string{"mcp.example.org"}
			addr := startGuarded(t, tr)
			auth := "Authorization: token decoy"
			if got := get(t, addr, tr.path, "Host: mcp.example.org", auth); strings.Contains(got, "403") {
				t.Errorf("declared host was rejected: %q", got)
			}
			if got := get(t, addr, tr.path, "Host: "+addr, auth); strings.Contains(got, "403") {
				t.Errorf("loopback host was rejected on a loopback listener: %q", got)
			}
			if got := get(t, addr, tr.path, "Host: other.example.org", auth); !strings.Contains(got, "403") {
				t.Errorf("undeclared host was accepted: %q", got)
			}
		})
	}
}

// --- Origin validation ---

func TestCrossSiteOriginIsRejectedEvenWithCorrectHost(t *testing.T) {
	// The case the library's own rebinding protection does not cover: a page on
	// any origin can address a loopback listener directly with a perfectly
	// correct Host header. Only the Origin header tells them apart.
	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			restoreFlags(t)
			addr := startGuarded(t, tr)
			got := get(t, addr, tr.path, "Host: "+addr, "Origin: https://attacker.example.com")
			if !strings.Contains(got, "403") {
				t.Fatalf("cross-site Origin was not rejected: %q", got)
			}
		})
	}
}

func TestAbsentOriginIsAccepted(t *testing.T) {
	// A non-browser client sends no Origin, which must not read as an attack.
	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			restoreFlags(t)
			addr := startGuarded(t, tr)
			if got := get(t, addr, tr.path, "Host: "+addr, "Authorization: token decoy"); strings.Contains(got, "403") {
				t.Fatalf("request with no Origin was rejected: %q", got)
			}
		})
	}
}

func TestPresentButEmptyOriginIsNotTreatedAsAbsent(t *testing.T) {
	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			restoreFlags(t)
			addr := startGuarded(t, tr)
			if got := get(t, addr, tr.path, "Host: "+addr, "Origin: "); !strings.Contains(got, "403") {
				t.Fatalf("present-but-empty Origin was accepted: %q", got)
			}
		})
	}
}

func TestOriginSchemeAndPortAreSignificant(t *testing.T) {
	// An Origin's port belongs to the requesting page, not to this listener, so
	// the port-insensitive rule that is correct for Host is wrong here. Reusing
	// one list for both would accept every value below.
	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			restoreFlags(t)
			flag.AllowedHosts = []string{"mcp.example.org"}
			flag.AllowedOrigins = []string{"https://console.example.org"}
			addr := startGuarded(t, tr)
			auth := "Authorization: token decoy"

			if got := get(t, addr, tr.path, "Host: mcp.example.org", "Origin: https://console.example.org", auth); strings.Contains(got, "403") {
				t.Errorf("the declared origin was rejected: %q", got)
			}
			// Same host, default port spelled out: still the same origin.
			if got := get(t, addr, tr.path, "Host: mcp.example.org", "Origin: https://console.example.org:443", auth); strings.Contains(got, "403") {
				t.Errorf("the declared origin with an explicit default port was rejected: %q", got)
			}
			for _, bad := range []string{
				"http://console.example.org",        // plaintext against an https origin
				"https://console.example.org:31337", // a different port is a different origin
				"https://mcp.example.org",           // a declared HOST is not a declared ORIGIN
				"https://evil.console.example.org",
				"null",
				"file:///etc/passwd",
				"https://user@console.example.org",
			} {
				if got := get(t, addr, tr.path, "Host: mcp.example.org", "Origin: "+bad, auth); !strings.Contains(got, "403") {
					t.Errorf("origin %q was accepted: %q", bad, got)
				}
			}
		})
	}
}

// --- Authentication at the door ---

func TestAnonymousRequestIsRefusedAtTheDoor(t *testing.T) {
	// Refusing at the forge client alone would still let an anonymous caller
	// open a session, enumerate the tool catalogue and hold an event stream.
	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			restoreFlags(t)
			addr := startGuarded(t, tr)
			got := get(t, addr, tr.path, "Host: "+addr)
			if !strings.Contains(got, "401") {
				t.Fatalf("anonymous request was not refused: %q", got)
			}
		})
	}
}

func TestOperatorFallbackOptInReAdmitsAnonymousRequests(t *testing.T) {
	// The explicit escape hatch, so a single-user deployment has an upgrade
	// path. It must work, and it must be the only thing that switches this on.
	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			restoreFlags(t)
			flag.AllowOperatorTokenFallback = true
			addr := startGuarded(t, tr)
			if got := get(t, addr, tr.path, "Host: "+addr); strings.Contains(got, "401") {
				t.Fatalf("the opt-in did not re-admit anonymous requests: %q", got)
			}
			if forgejo.RequireRequestToken() {
				t.Fatal("the opt-in did not reach the credential policy")
			}
		})
	}
}

func TestAuthorizationHeaderShapesThatCarryNoTokenAreRefused(t *testing.T) {
	restoreFlags(t)
	addr := startGuarded(t, transports()[1])
	for _, header := range []string{
		"Authorization: ",
		"Authorization: token",
		"Authorization: token ",
		"Authorization: Bearer",
		"Authorization: Bearer ",
		"Authorization: Basic abc",
		"Authorization: Negotiate abc",
	} {
		if got := get(t, addr, streamableHTTPEndpointPath, "Host: "+addr, header); !strings.Contains(got, "401") {
			t.Errorf("%q was accepted as an identity: %q", header, got)
		}
	}
}

// --- The reverse-proxy bypass ---

func TestDefaultReverseProxyDoesNotReAdmitTheOperatorCredential(t *testing.T) {
	// nginx and Apache both rewrite Host to the proxied target by default, so a
	// request from the internet arrives on a loopback socket carrying a
	// loopback Host. Inferring "local, therefore the operator's own credential
	// may stand in" from those observables would serve a remote anonymous
	// caller with the operator's forge credential.
	//
	// The credential policy does not read the bind address at all, so the
	// proxied request is refused for want of a token. This test exists to keep
	// it that way.
	restoreFlags(t)
	tr := transports()[1]
	addr := startGuarded(t, tr)

	target, err := url.Parse("http://" + addr)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		r.Host = target.Host // nginx's documented default: Host = $proxy_host
	}
	front := httptest.NewServer(proxy)
	t.Cleanup(front.Close)

	resp, err := front.Client().Get(front.URL + streamableHTTPEndpointPath)
	if err != nil {
		t.Fatalf("through the proxy: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("an anonymous request through a default-configured reverse proxy got %d, want 401", resp.StatusCode)
	}
	if forgejo.RequireRequestToken() != true {
		t.Fatal("the credential fallback was live on a network transport")
	}
}

// --- Request-shape bypasses ---

func TestOptionsStarDoesNotSkipTheGuard(t *testing.T) {
	// Go replaces srv.Handler entirely for "OPTIONS *" unless
	// DisableGeneralOptionsHandler is set, routing that one request shape
	// around every check.
	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			restoreFlags(t)
			addr := startGuarded(t, tr)
			got := rawRequest(t, addr, "OPTIONS * HTTP/1.1", "Host: attacker.example.com")
			if !strings.Contains(got, "403") {
				t.Fatalf("OPTIONS * with a forged Host was not rejected: %q", got)
			}
		})
	}
}

func TestOversizedHeaderIsRefusedRatherThanLogged(t *testing.T) {
	// The refusal path logs the rejected header. Without a header cap, a
	// stranger can write to the operator's disk for free using the very
	// requests this server refuses.
	restoreFlags(t)
	tr := transports()[1]
	addr := startGuarded(t, tr)
	big := strings.Repeat("a", 200_000) + ".attacker.example.com"
	got := get(t, addr, streamableHTTPEndpointPath, "Host: "+big)
	if !strings.Contains(got, "431") && !strings.Contains(got, "400") {
		t.Fatalf("a 200 KB Host header was not refused by the header cap: %q", got)
	}
}

func TestTruncateForLogBoundsAttackerInput(t *testing.T) {
	if got := truncateForLog(strings.Repeat("a", 5000)); len(got) > maxLoggedHeaderLen+32 {
		t.Fatalf("logged value not bounded: %d bytes", len(got))
	}
	if got := truncateForLog("short"); got != "short" {
		t.Fatalf("short value was altered: %q", got)
	}
}

func TestTransportAnswersOnlyItsOwnPath(t *testing.T) {
	// Serving the transport as the root handler instead of mounting it makes it
	// answer on every path, which widens the surface without saying so.
	restoreFlags(t)
	tr := transports()[1]
	addr := startGuarded(t, tr)
	auth := "Authorization: token decoy"
	if got := get(t, addr, "/", "Host: "+addr, auth); !strings.Contains(got, "404") {
		t.Errorf("the transport answered on /: %q", got)
	}
	if got := get(t, addr, "/.well-known/anything", "Host: "+addr, auth); !strings.Contains(got, "404") {
		t.Errorf("the transport answered on an arbitrary path: %q", got)
	}
}

// --- Configuration ---

func TestExposedListenerWithNoDeclaredHostsRefusesBeforeBinding(t *testing.T) {
	restoreFlags(t)
	flag.Host = "0.0.0.0"
	flag.AllowedHosts = nil
	_, err := resolveTransportConfig("http")
	if err == nil {
		t.Fatal("a network-reachable configuration with no declared hosts was allowed")
	}
	if !strings.Contains(err.Error(), "allowed-hosts") {
		t.Fatalf("refusal does not name the option that fixes it: %v", err)
	}
}

func TestLoopbackConfigurationBindsBothLoopbackFamilies(t *testing.T) {
	// Binding only 127.0.0.1 leaves a client that resolves "localhost" to ::1
	// unable to connect — and connecting to "localhost" is what this project's
	// own documentation tells clients to do.
	restoreFlags(t)
	for _, host := range []string{"127.0.0.1", "localhost", "::1"} {
		flag.Host = host
		cfg, err := resolveTransportConfig("http")
		if err != nil {
			t.Fatalf("%s: %v", host, err)
		}
		if len(cfg.listenHosts) != 2 {
			t.Errorf("%s: listens on %v, want both loopback families", host, cfg.listenHosts)
		}
	}
}

func TestCredentialPolicyIgnoresTheBindAddress(t *testing.T) {
	restoreFlags(t)
	for _, host := range []string{"127.0.0.1", "0.0.0.0"} {
		flag.Host = host
		flag.AllowedHosts = []string{"mcp.example.org"}
		cfg, err := resolveTransportConfig("http")
		if err != nil {
			t.Fatalf("%s: %v", host, err)
		}
		if !cfg.requireAuth {
			t.Errorf("%s: a network transport must require a per-request credential", host)
		}
	}
	flag.AllowOperatorTokenFallback = true
	cfg, err := resolveTransportConfig("http")
	if err != nil {
		t.Fatalf("with the opt-in: %v", err)
	}
	if cfg.requireAuth {
		t.Error("the explicit opt-in did not take effect")
	}
}

func TestMalformedAllowedOriginRefusesToStart(t *testing.T) {
	restoreFlags(t)
	for _, bad := range []string{"console.example.org", "null", "ftp://x", "https://"} {
		flag.AllowedOrigins = []string{bad}
		if _, err := resolveTransportConfig("http"); err == nil {
			t.Errorf("%q was accepted as an origin", bad)
		}
	}
}

func TestAddrIsLoopbackOnly(t *testing.T) {
	cases := []struct {
		addr net.Addr
		want bool
	}{
		{&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 1}, true},
		{&net.TCPAddr{IP: net.ParseIP("127.0.0.53"), Port: 1}, true},
		{&net.TCPAddr{IP: net.ParseIP("::1"), Port: 1}, true},
		// The unspecified address is what a bare ":port" produces. It is
		// reachable from the network and must not read as loopback — this is
		// the original defect.
		{&net.TCPAddr{IP: net.IPv4zero, Port: 1}, false},
		{&net.TCPAddr{IP: net.IPv6unspecified, Port: 1}, false},
		{&net.TCPAddr{IP: net.ParseIP("192.168.1.10"), Port: 1}, false},
		{&net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 1}, false},
		{&net.UnixAddr{Name: "/tmp/x"}, false},
	}
	for _, c := range cases {
		if got := addrIsLoopbackOnly(c.addr); got != c.want {
			t.Errorf("addrIsLoopbackOnly(%v) = %v, want %v", c.addr, got, c.want)
		}
	}
}

func TestHostPolicyPermits(t *testing.T) {
	loopbackDefault := hostPolicy{loopbackOnly: true}
	declared := hostPolicy{allowed: []string{"mcp.example.org", "pinned.example.org:8443", "::1"}}

	cases := []struct {
		policy hostPolicy
		value  string
		want   bool
	}{
		{loopbackDefault, "127.0.0.1:8080", true},
		{loopbackDefault, "localhost:8080", true},
		{loopbackDefault, "LOCALHOST:8080", true},
		{loopbackDefault, "[::1]:8080", true},
		{loopbackDefault, "attacker.example.com", false},
		{loopbackDefault, "attacker.example.com:8080", false},
		{loopbackDefault, "localhost.attacker.example.com", false},
		{loopbackDefault, "127.0.0.1.attacker.example.com", false},
		{loopbackDefault, "", false},
		// A port that is not a number must not be parsed leniently: splitting
		// on the last colon would otherwise reduce these to an accepted host.
		{loopbackDefault, "127.0.0.1:8080@attacker.example.com", false},
		{loopbackDefault, "127.0.0.1:", false},
		{declared, "mcp.example.org", true},
		{declared, "mcp.example.org:8080", true},
		{declared, "pinned.example.org:8443", true},
		{declared, "pinned.example.org:9999", false},
		{declared, "pinned.example.org", false},
		{declared, "evil.mcp.example.org", false},
		{declared, "mcp.example.org:not-a-port", false},
		// An IPv6 entry must be usable, bracketed or not.
		{declared, "[::1]:8080", true},
		{declared, "[::1]", true},
		// Loopback is not implicitly allowed when the listener is NOT
		// loopback-only, or a stranger could send Host: localhost.
		{declared, "127.0.0.1:8080", false},
	}
	for _, c := range cases {
		if got := c.policy.permits(c.value); got != c.want {
			t.Errorf("permits(%q) = %v, want %v", c.value, got, c.want)
		}
	}
}

func TestNormalizeOrigin(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"https://a.example.org", "https://a.example.org", true},
		{"https://a.example.org:443", "https://a.example.org", true},
		{"http://a.example.org:80", "http://a.example.org", true},
		{"HTTPS://A.EXAMPLE.ORG", "https://a.example.org", true},
		{"https://a.example.org:8443", "https://a.example.org:8443", true},
		{"http://[::1]:3000", "http://::1:3000", true},
		{"null", "", false},
		{"a.example.org", "", false},
		{"file:///etc/passwd", "", false},
		{"https://user@a.example.org", "", false},
		{"https://", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := normalizeOrigin(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("normalizeOrigin(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestNoTransportCallsTheLibrarysOwnStart(t *testing.T) {
	// A transport that binds its own listener misses the bind address, the
	// header checks and the credential policy at once, and nothing else in this
	// suite would notice. Assert on the source.
	src := mustReadSource(t, "operation.go")
	for _, forbidden := range []string{"sseServer.Start(", "httpServer.Start("} {
		if strings.Contains(src, forbidden) {
			t.Errorf("operation.go still calls %s — every transport must go through serveMCPOverHTTP", forbidden)
		}
	}
}

func TestEveryNetworkTransportIsCovered(t *testing.T) {
	// The table above is what makes every conformance case run against both
	// transports. A contributor who adds a third — or quietly deletes a row —
	// would otherwise reduce coverage with nothing turning red.
	src := mustReadSource(t, "operation.go")
	covered := map[string]bool{}
	for _, tr := range transports() {
		covered[tr.name] = true
	}
	for _, name := range []string{"stdio", "sse", "http"} {
		if !strings.Contains(src, fmt.Sprintf("case %q:", name)) {
			t.Errorf("operation.go no longer implements the %q transport — update this test", name)
		}
	}
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, `case "`) || !strings.HasSuffix(line, `":`) {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(line, `case "`), `":`)
		if name == "stdio" || covered[name] {
			continue
		}
		t.Errorf("transport %q is implemented in operation.go but not covered by the conformance table", name)
	}
}

func TestPerRequestCredentialReachesTheHandlerContext(t *testing.T) {
	// The door check reads the Authorization header directly, so it would still
	// pass if the context function that lifts the credential into the request
	// context were broken. The failure would surface only as every
	// authenticated call failing at the forge client — which no other test
	// here would see.
	restoreFlags(t)
	forgejo.SetRequireRequestToken(true)

	req, err := http.NewRequest(http.MethodPost, "http://example.invalid/mcp", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "token decoy-per-request-value")

	// Assert on the credential decision only. Building the client can fail for
	// unrelated reasons here (no forge URL is configured in a unit test), and
	// that is not what this test is about.
	ctx := requestTokenContextFunc(req.Context(), req)
	if _, err := forgejo.Client(ctx); errors.Is(err, forgejo.ErrNoRequestToken) {
		t.Fatal("a request carrying a credential was refused for want of one")
	}

	// The control: with no header, the same path must refuse.
	bare, err := http.NewRequest(http.MethodPost, "http://example.invalid/mcp", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if _, err := forgejo.Client(requestTokenContextFunc(bare.Context(), bare)); !errors.Is(err, forgejo.ErrNoRequestToken) {
		t.Fatalf("a request carrying no credential was not refused: %v", err)
	}
}

func TestAuthenticatedRequestPassesTheDoor(t *testing.T) {
	// End-to-end control for the 401 tests: if the door refused everything,
	// those would pass while the server was useless.
	for _, tr := range transports() {
		t.Run(tr.name, func(t *testing.T) {
			restoreFlags(t)
			addr := startGuarded(t, tr)
			got := get(t, addr, tr.path, "Host: "+addr, "Authorization: Bearer decoy-per-request-value")
			if strings.Contains(got, "401") || strings.Contains(got, "403") {
				t.Fatalf("an authenticated request was refused: %q", got)
			}
		})
	}
}
