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

	Debug bool
)
