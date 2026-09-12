<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Tasks

- [x] `--host` / `FORGEJO_MCP_HOST`, default `localhost`; a loopback NAME binds both
      loopback families so a client resolving `localhost` to `::1` still connects, and
      a loopback ADDRESS binds that family alone.
- [x] A loopback family is skipped only when this machine cannot use it
      (`EADDRNOTAVAIL`, `EAFNOSUPPORT`), whichever family it is; any other bind
      failure on either family, including a port already in use, refuses to start.
      Naming one address is the way out when a stack fails with an errno the check
      cannot recognise, and a refusal on a name says so and names the setting.
- [x] `--allowed-hosts` / `FORGEJO_MCP_ALLOWED_HOSTS`; required for a non-loopback
      bind, validated before anything is bound so a misconfigured start never opens a
      public socket.
- [x] `--allowed-origins` / `FORGEJO_MCP_ALLOWED_ORIGINS`, compared as full origins
      with default-port normalisation, separate from the host list.
- [x] `--allow-operator-token-fallback`, off by default.
- [x] Flag-versus-environment precedence resolved with `FlagSet.Visit`, so a flag
      explicitly set to its default value is not mistaken for an unset one.
- [x] `serveMCPOverHTTP` owns the listeners for both transports; neither calls the
      library's `Start`. Streamable HTTP mounted on a mux at its endpoint path.
- [x] `guardRequests` validates `Host`, then `Origin`, then `Authorization`.
      `DisableGeneralOptionsHandler` set, because Go replaces the server's handler
      entirely for `OPTIONS *`.
- [x] Request bounds on a listener strangers can reach: header cap, read, write and
      idle timeouts, and truncation of attacker-controlled values before they are
      logged.
- [x] `tokenForRequest` is the one place the credential fallback lives; both the typed
      client factory and the raw-HTTP helper call it.
- [x] Conformance tests attack a running server over raw sockets, run every case
      against both transports from one table, and assert the table covers every
      transport the switch implements.
- [x] Every control mutation-tested: 21 deliberate breakages, each caught by a named
      test, against a green control run.
