<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Security Policy

## Reporting a vulnerability

**Please do not open a public issue for a security problem.** The issue tracker
is world-readable, and a report there is a disclosure whether or not it was
meant as one.

Email **<goern@b4mad.net>** with the subject line prefixed `[forgejo-mcp
security]`. That prefix is what gets it out of a normal inbox and into the
right pile.

If you would rather not use email, open a normal issue that says only *"I have
a security report, how should I send it?"* — no details — and you will get a
private channel back. An empty placeholder issue discloses nothing.

### What to include

The more of this you have, the faster it moves. Send what you have; a partial
report now beats a complete one in three weeks.

- What an attacker gains — read the operator's forge token, write to a repo
  they should not reach, run code, deny service.
- Which transport (`stdio`, `sse`, `http`) and roughly which version or commit.
- The configuration that exposes it, especially anything non-default.
- Minimal steps to reproduce. A `curl` invocation or a short script is ideal.
- Whether you have told anyone else, and whether you have a disclosure date in
  mind.

You do not need a working exploit, and please do not attach one against a forge
you do not own.

## What to expect

This project is maintained by a small group in and around working hours across
European time zones. The targets below are what we commit to, not a
best-imaginable case:

| Stage | Target |
| --- | --- |
| Acknowledgement that a human has read it | 5 working days |
| Initial assessment — confirmed or not, rough severity | 10 working days |
| Fix released, for a confirmed issue | Depends on severity; discussed with you |

If you have not heard anything after **10 working days**, assume the mail went
astray rather than that it was ignored — send it again, and mention in a public
issue that you are waiting on a security contact (still without details).
That escalation path is deliberate: a report that silently fails to arrive is
the worst outcome for everyone.

Being honest about the record here: a report in August 2026 sat for over two
weeks before the reporter got a substantive answer. This policy exists so the
next one does not.

## Coordinated disclosure

- We will agree a disclosure date with you. Absent a reason to differ, the
  default is **90 days from the acknowledgement**, or the day the fix ships,
  whichever comes first.
- We will not ask you to stay quiet indefinitely. If we cannot fix something in
  90 days we will say so and explain why, and you remain free to publish.
- You will be credited by whatever name and link you choose, in the release
  notes and the fixing pull request. Tell us if you would rather not be.
- We do not run a bug bounty and cannot offer payment.

## Scope

**In scope** — this repository: the MCP server binary, its transports, its
handling of forge credentials, the tool and resource surface, and the release
artifacts published from this repo (binaries, SBOMs, signatures, container
images, `.mcpb` bundles).

**Out of scope**, though we would still like to hear about the first two:

- Vulnerabilities in Forgejo or Gitea themselves — report those to
  [Forgejo](https://codeberg.org/forgejo/forgejo) upstream.
- Vulnerabilities in the Go SDK or other third-party dependencies — report
  upstream; tell us too, so we can pin or patch.
- The `git.b4mad.industries` forge instance, our CI, or any other b4mad
  infrastructure. Those are separate systems with separate operators.
- Findings that require an attacker who already has the operator's forge token,
  shell access as the server's user, or the ability to modify its
  configuration. Those are not boundaries this server defends.

## Supported versions

Only the **latest release** is supported. There are no maintenance branches and
no backports: fixes land on `main` and ship in the next release, which is
usually days rather than months.

| Version | Supported |
| --- | --- |
| Latest `v2.x` release | ✅ |
| Any earlier release | ❌ — upgrade |

## Deployment notes

Two properties of this server are worth understanding before you expose it,
because they shape what a vulnerability report even means:

1. **The server holds a forge access token** with whatever permissions you
   granted it. Anything that reaches the server's request path is a step toward
   that credential. Scope the token to the minimum the tools you actually use
   require.
2. **`stdio` and the network transports have very different exposure.** Under
   `stdio` the client launches the process and nothing listens on a socket.
   Under `--transport sse` or `--transport http` the server accepts network
   requests, and the security of the deployment depends on the bind address,
   the request authentication and whatever sits in front of it. Read the
   transport section of the [README](README.md) before running either, and
   treat the port as sensitive.

Reports that a network transport is reachable or misconfigured *in your own
deployment* are support questions, not vulnerabilities — the tracker is the
right place for those. Reports that the server behaves less safely than it
documents are vulnerabilities, and we want them.
