package flag

var (
	URL       string
	SSEPort   int
	HTTPPort  int
	Token     string
	Version   string
	UserAgent string

	// Host is the address the network transports bind to. It defaults to
	// loopback, per the Model Context Protocol's guidance that a locally-run
	// server "SHOULD bind only to localhost (127.0.0.1) rather than all
	// network interfaces". An operator who wants the multi-client topology
	// sets it deliberately.
	Host string

	// AllowedHosts lists the Host values a network-reachable listener answers
	// to. It is required when Host is not loopback. Loopback names are always
	// accepted on a loopback-only listener, whatever is declared here.
	AllowedHosts []string

	// AllowedOrigins lists the web origins this server accepts an Origin
	// header from, as full origins ("https://console.example.org"). Empty — the
	// default — means no browser origin is accepted, which is correct for a
	// server whose clients are not browsers. A request carrying no Origin at
	// all is unaffected.
	//
	// This is deliberately separate from AllowedHosts. A Host names this
	// server; an Origin names the page making the request. One list for both
	// silently accepts every other service sharing a declared hostname on any
	// port.
	AllowedOrigins []string

	// AllowOperatorTokenFallback re-enables serving a request that carries no
	// Authorization header using this server's own configured credential, on
	// the sse and http transports. It is off by default and exists only so an
	// operator running a single-user deployment has an upgrade path. On stdio
	// the fallback is always available and this setting is irrelevant.
	AllowOperatorTokenFallback bool

	// AuthMode selects how the http transport authenticates callers:
	// "passthrough" (the default) forwards the caller's Authorization header to
	// Forgejo; "resource-server" validates an identity provider's JWT access
	// token and signs a Forgejo Authorized Integration token instead.
	AuthMode string

	// The settings below apply only to AuthMode "resource-server".

	// AuthorizationServer is the identity provider's issuer URL.
	AuthorizationServer string
	// Resource is the canonical URI of this server's MCP endpoint.
	Resource string
	// ResourceAudience is the value an inbound token's aud must contain. Empty
	// means Resource.
	ResourceAudience string
	// ScopesSupported lists the scopes advertised to clients.
	ScopesSupported []string
	// ForgejoAudienceClaim names the inbound claim carrying the caller's Forgejo
	// integration audience.
	ForgejoAudienceClaim string
	// ForgejoJWTIssuer is the issuer URL this server presents to Forgejo.
	ForgejoJWTIssuer string
	// ForgejoJWTSigningKeyFile is the PEM private key Forgejo tokens are signed
	// with.
	ForgejoJWTSigningKeyFile string
	// ForgejoJWTPublishedKeyFiles are PEM keys published without signing with
	// them, for rotation.
	ForgejoJWTPublishedKeyFiles []string

	// ResourceServerSettings names every resource-server-only setting that was
	// given at all, as a flag or an environment variable. passthrough mode
	// refuses to start when it is not empty.
	ResourceServerSettings []string

	Debug bool
)
