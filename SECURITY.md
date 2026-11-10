# Security policy

## Supported versions

Fixes go into the latest minor release of each module: the core (`vX.Y.Z`), `grpcvalidator/vX.Y.Z` and `playground/vX.Y.Z`. Until 1.0.0, older minor versions are not patched; upgrade to the latest one.

## Reporting a vulnerability

Please do not open a public issue for a security problem. Report it privately through GitHub: open the repository's **Security** tab and choose **Report a vulnerability**. Include the version, the rules and input that trigger the problem, and what happens.

You will get an answer within a week. Once a fix is released, the advisory is published with credit to the reporter, unless they prefer otherwise.

## What counts

This package checks untrusted input, so these are security problems:

- input that passes a rule it should fail, such as a value outside `between`;
- input that makes validation or the adapters (`nethttp`, `grpcvalidator`) panic, hang, or use far more time or memory than the size of the request;
- error messages or responses that reveal server internals.

Rule strings, struct tags and custom rules are written by the application, not by its users, so a problem that needs an attacker to control them is not a vulnerability in this package. For how the package is tested, see [docs/SECURITY_LAB.md](docs/SECURITY_LAB.md).
