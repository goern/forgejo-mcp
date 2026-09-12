// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

const (
	v4Loopback = "127.0.0.1:8080"
	v6Loopback = "[::1]:8080"
)

// fakeListener stands in for a bound socket and records whether it was closed,
// so a test can require that a refused start leaves nothing bound behind.
type fakeListener struct {
	addr   net.Addr
	closed bool
}

func (l *fakeListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (l *fakeListener) Close() error              { l.closed = true; return nil }
func (l *fakeListener) Addr() net.Addr            { return l.addr }

// scriptedListen fails the addresses in failures with the given errno, wrapped
// the way net.Listen wraps a real bind failure, and binds every other address.
type scriptedListen struct {
	failures map[string]syscall.Errno
	bound    []*fakeListener
}

func (s *scriptedListen) listen(network, address string) (net.Listener, error) {
	if errno, ok := s.failures[address]; ok {
		return nil, &net.OpError{Op: "listen", Net: network, Err: os.NewSyscallError("bind", errno)}
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}
	ln := &fakeListener{addr: &net.TCPAddr{IP: net.ParseIP(host), Port: p}}
	s.bound = append(s.bound, ln)
	return ln, nil
}

func (s *scriptedListen) requireAllClosed(t *testing.T) {
	t.Helper()
	for _, ln := range s.bound {
		if !ln.closed {
			t.Errorf("listener on %s was left open after a refused start", ln.addr)
		}
	}
}

// loopbackConfig resolves the default configuration: the loopback NAME, which
// binds both families.
func loopbackConfig(t *testing.T) transportConfig {
	t.Helper()
	restoreFlags(t)
	cfg, err := resolveTransportConfig("http")
	if err != nil {
		t.Fatalf("resolveTransportConfig: %v", err)
	}
	return cfg
}

// literalConfig resolves a configuration that names one loopback address.
func literalConfig(t *testing.T, host string) transportConfig {
	t.Helper()
	restoreFlags(t)
	flag.Host = host
	cfg, err := resolveTransportConfig("http")
	if err != nil {
		t.Fatalf("resolveTransportConfig(%s): %v", host, err)
	}
	return cfg
}

// observeLogs routes this package's log output into memory for one test.
func observeLogs(t *testing.T) *observer.ObservedLogs {
	t.Helper()
	core, logs := observer.New(zapcore.DebugLevel)
	prev := log.Default()
	log.SetDefault(zap.New(core))
	t.Cleanup(func() { log.SetDefault(prev) })
	return logs
}

func boundAddrs(listeners []net.Listener) []string {
	out := make([]string, 0, len(listeners))
	for _, ln := range listeners {
		out = append(out, ln.Addr().String())
	}
	return out
}

func TestUnavailableLoopbackFamilyDoesNotBlockStartup(t *testing.T) {
	// A host with IPv6 disabled answers EADDRNOTAVAIL for ::1; one without IPv6
	// in the kernel answers EAFNOSUPPORT. Either family may be the missing one,
	// and which of them is tried first must not decide whether the server
	// starts: the previous implementation tolerated the second family failing
	// but not the first.
	cases := []struct {
		name      string
		missing   string
		errno     syscall.Errno
		remaining string
	}{
		{"IPv6 disabled", v6Loopback, syscall.EADDRNOTAVAIL, v4Loopback},
		{"IPv6 absent from the kernel", v6Loopback, syscall.EAFNOSUPPORT, v4Loopback},
		{"IPv4 loopback unavailable, tried first", v4Loopback, syscall.EADDRNOTAVAIL, v6Loopback},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := loopbackConfig(t)
			logs := observeLogs(t)
			s := &scriptedListen{failures: map[string]syscall.Errno{c.missing: c.errno}}

			listeners, err := bindListeners("http", cfg, 8080, s.listen)
			if err != nil {
				t.Fatalf("startup was refused: %v", err)
			}
			if got := boundAddrs(listeners); len(got) != 1 || got[0] != c.remaining {
				t.Fatalf("bound %v, want only %s", got, c.remaining)
			}
			// The skip must be visible at the default level. It used to be
			// logged at Debug, which an operator does not see.
			skipped := logs.FilterMessageSnippet("family unavailable").All()
			if len(skipped) != 1 || skipped[0].Level < zapcore.InfoLevel {
				t.Fatalf("the skipped family was not logged at Info or above: %v", skipped)
			}
		})
	}
}

func TestTakenLoopbackPortRefusesToStart(t *testing.T) {
	// A port held by another process, or forbidden, on EITHER family must stop
	// startup. Serving on the other family alone would send every client that
	// resolves "localhost" to the taken family -- Authorization header included --
	// to whatever holds that port.
	for _, errno := range []syscall.Errno{syscall.EADDRINUSE, syscall.EACCES} {
		for _, taken := range []string{v4Loopback, v6Loopback} {
			t.Run(fmt.Sprintf("%s on %s", errno, taken), func(t *testing.T) {
				cfg := loopbackConfig(t)
				s := &scriptedListen{failures: map[string]syscall.Errno{taken: errno}}

				listeners, err := bindListeners("http", cfg, 8080, s.listen)
				if err == nil {
					t.Fatalf("started on %v although %s failed with %v", boundAddrs(listeners), taken, errno)
				}
				if !errors.Is(err, errno) {
					t.Errorf("the refusal does not carry its cause: %v", err)
				}
				s.requireAllClosed(t)
			})
		}
	}
}

func TestNoUsableLoopbackFamilyRefusesToStart(t *testing.T) {
	cfg := loopbackConfig(t)
	s := &scriptedListen{failures: map[string]syscall.Errno{
		v4Loopback: syscall.EADDRNOTAVAIL,
		v6Loopback: syscall.EAFNOSUPPORT,
	}}
	listeners, err := bindListeners("http", cfg, 8080, s.listen)
	if err == nil {
		t.Fatalf("started with no loopback family available, on %v", boundAddrs(listeners))
	}
	// The refusal carries what each family said. Reporting only "failed to bind
	// any listener" leaves a machine with no loopback at all indistinguishable
	// from one whose families were never tried.
	for _, errno := range []syscall.Errno{syscall.EADDRNOTAVAIL, syscall.EAFNOSUPPORT} {
		if !errors.Is(err, errno) {
			t.Errorf("the refusal dropped %v: %v", errno, err)
		}
	}
}

func TestLoopbackAddressBindsOnlyItselfEvenWhenTheOtherFamilyIsTaken(t *testing.T) {
	// The escape hatch. isFamilyUnavailable allowlists two errnos, and a stack
	// that fails with a third one would otherwise leave the operator with a
	// server that refuses to start and no flag that makes it start. Naming one
	// address means the other family is never touched, whatever it would answer.
	cfg := literalConfig(t, "::1")
	s := &scriptedListen{failures: map[string]syscall.Errno{
		v4Loopback: syscall.EPROTONOSUPPORT, // an errno the allowlist does not know
	}}

	listeners, err := bindListeners("http", cfg, 8080, s.listen)
	if err != nil {
		t.Fatalf("startup was refused although only the unnamed family failed: %v", err)
	}
	if got := boundAddrs(listeners); len(got) != 1 || got[0] != v6Loopback {
		t.Fatalf("bound %v, want only %s", got, v6Loopback)
	}
}

func TestRefusalOnALoopbackNameNamesTheSingleFamilyRemedy(t *testing.T) {
	// A loopback name expands to two addresses, so the failing one can be an
	// address the operator never wrote down. The refusal has to say that, and
	// say which flag binds one family.
	nameCfg := loopbackConfig(t)
	s := &scriptedListen{failures: map[string]syscall.Errno{v6Loopback: syscall.EADDRINUSE}}
	_, err := bindListeners("http", nameCfg, 8080, s.listen)
	if err == nil {
		t.Fatal("a taken loopback port was allowed to start")
	}
	for _, want := range []string{"binds both", "-host 127.0.0.1", "-host ::1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q: %v", want, err)
		}
	}

	// An operator who named the address already knows which family it is.
	literal := literalConfig(t, "::1")
	s = &scriptedListen{failures: map[string]syscall.Errno{v6Loopback: syscall.EADDRINUSE}}
	_, err = bindListeners("http", literal, 8080, s.listen)
	if err == nil {
		t.Fatal("a taken port on the named address was allowed to start")
	}
	if strings.Contains(err.Error(), "binds both") {
		t.Errorf("the refusal offers a remedy the operator already applied: %v", err)
	}
}

func TestUnavailableAddressIsFatalOffLoopback(t *testing.T) {
	// Skipping a family is for the second loopback family only. An operator who
	// named an address this machine does not have must be told, not ignored.
	restoreFlags(t)
	flag.Host = "192.0.2.10"
	flag.AllowedHosts = []string{"mcp.example.org"}
	cfg, err := resolveTransportConfig("http")
	if err != nil {
		t.Fatalf("resolveTransportConfig: %v", err)
	}
	s := &scriptedListen{failures: map[string]syscall.Errno{"192.0.2.10:8080": syscall.EADDRNOTAVAIL}}
	if _, err := bindListeners("http", cfg, 8080, s.listen); err == nil {
		t.Fatal("started although the one configured address could not be bound")
	}
}

func TestLoopbackConfigurationThatBindsElsewhereRefusesToStart(t *testing.T) {
	// The bound address is checked, not the configured string.
	cfg := loopbackConfig(t)
	var bound []*fakeListener
	listen := func(network, address string) (net.Listener, error) {
		ln := &fakeListener{addr: &net.TCPAddr{IP: net.IPv4zero, Port: 8080}}
		bound = append(bound, ln)
		return ln, nil
	}
	if _, err := bindListeners("http", cfg, 8080, listen); err == nil {
		t.Fatal("a loopback configuration that bound the unspecified address was allowed to start")
	}
	for _, ln := range bound {
		if !ln.closed {
			t.Errorf("listener on %s was left open after a refused start", ln.addr)
		}
	}
}

func TestDefaultConfigurationBindsLoopbackOnly(t *testing.T) {
	// Real sockets. Port 0 lets the kernel choose, and it chooses separately per
	// family; that is harmless here, since this asserts where the listeners are
	// and not which port they share.
	cfg := loopbackConfig(t)
	listeners, err := bindListeners("http", cfg, 0, net.Listen)
	if err != nil {
		t.Fatalf("bindListeners: %v", err)
	}
	t.Cleanup(func() {
		for _, ln := range listeners {
			_ = ln.Close()
		}
	})
	for _, ln := range listeners {
		if !addrIsLoopbackOnly(ln.Addr()) {
			t.Errorf("the default configuration bound %s, which is not loopback", ln.Addr())
		}
	}
}

func TestStartupLogSaysLoopbackIsThisMachineOnly(t *testing.T) {
	cfg := loopbackConfig(t)
	logs := observeLogs(t)

	logListening("http", []string{v4Loopback, v6Loopback}, cfg)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("got %d startup log lines, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if got := fields["address"]; got != v4Loopback+", "+v6Loopback {
		t.Errorf("address = %v, want the addresses actually bound", got)
	}
	if got, _ := fields["reachable_from"].(string); !strings.HasPrefix(got, "this machine only") {
		t.Errorf("reachable_from = %q, want it to say this machine only", got)
	}
	if got, _ := fields["authentication"].(string); !strings.Contains(got, "must carry its own Authorization header") {
		t.Errorf("authentication = %q, want the per-request requirement stated", got)
	}
	if line := entries[0].Message + fmt.Sprint(fields); strings.Contains(line, "http://localhost") {
		t.Errorf("the startup log prints a fixed localhost URL: %s", line)
	}
}

func TestStartupLogAnnouncesTheCredentialFallback(t *testing.T) {
	restoreFlags(t)
	flag.AllowOperatorTokenFallback = true
	cfg, err := resolveTransportConfig("http")
	if err != nil {
		t.Fatalf("resolveTransportConfig: %v", err)
	}
	logs := observeLogs(t)

	logListening("http", []string{v4Loopback}, cfg)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("got %d startup log lines, want 1", len(entries))
	}
	if entries[0].Level < zapcore.WarnLevel {
		t.Errorf("the fallback was announced at %v, want Warn or above", entries[0].Level)
	}
	if got, _ := entries[0].ContextMap()["authentication"].(string); !strings.Contains(got, "served using this server's own credential") {
		t.Errorf("authentication = %q, want it to say anonymous requests use this server's credential", got)
	}
}
