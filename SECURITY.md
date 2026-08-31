# Security policy

Env Setup Wizard handles configuration values that may be sensitive and writes
files used by other applications. Please report security problems privately so
they can be investigated before details are made public.

## Supported versions

Published versions follow this support policy:

| Version | Security support |
| --- | --- |
| Latest stable v1 release | Supported |
| Latest v1 release candidate, if no stable release exists | Supported |
| Older releases and arbitrary source snapshots | Not supported |

This table may be revised when the project has enough users to justify multiple
simultaneously supported release lines.

## Reporting a vulnerability

Use the repository's private **Report a vulnerability** flow in the GitHub
Security tab. If that flow is unavailable, open a public issue containing only
a request for a private contact channel. Do not include vulnerability details,
proof-of-concept material, credentials, or configuration values in that issue.

Include as much of the following as is safe:

- affected version or commit;
- operating system and architecture;
- impact and the conditions required to trigger it;
- minimal reproduction steps or a sanitized template;
- whether the problem exposes secrets, changes an unintended file, loses data,
  or breaks backup/atomic-write guarantees;
- any proposed mitigation or fix.

Never send a real `.env`, backup, token, password, or private key. Replace secret
values and identifying paths before attaching diagnostics.

## Relevant security issues

Examples that belong in a private report include:

- an `@secret` value appearing in terminal output, an error, or a summary;
- writing through a rejected symlink or replacing a file other than the selected output;
- losing or corrupting the previous output during an overwrite failure;
- bypassing template validation in a way that causes command execution or reads
  values from the ambient process environment;
- a dependency vulnerability that is reachable through this command;
- a release artifact or checksum that does not match the documented build process.

## Expected behavior and limitations

The following are documented properties rather than vulnerabilities by themselves:

- `.env` and timestamped backups contain secret values in plaintext;
- `@secret` masks terminal presentation but is not encryption or a secret store;
- users are responsible for file access controls, repository ignore rules, and
  removal of backups they no longer need;
- exact ownership, ACL, timestamp, and other metadata preservation is not guaranteed;
- atomicity and durability can be weaker on network shares or unusual filesystems;
- an attacker who already controls the same operating-system account can read or
  replace files that account can access.

Reports showing a concrete impact beyond these stated limitations are still
welcome. Questions and ordinary bugs that contain no sensitive security details
can use the public issue tracker.

## Disclosure process

The maintainer will validate the report, assess affected versions, prepare tests
and a fix, and coordinate disclosure with the reporter. There is currently no
guaranteed response-time SLA, but reports will be handled as promptly as the
project's maintainer capacity allows.

Please avoid public disclosure until a fix or mitigation is available and a
coordinated publication date has been agreed. Credit will be given in the
advisory when desired and mutually agreed.

When official release artifacts become available, obtain them from this
repository's GitHub Releases page and verify them against the published
`SHA256SUMS` file before use.
