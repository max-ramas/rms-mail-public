# Contributing to RMS Mail (Mono edition)

Thanks for your interest in contributing! This document covers what you need to know before opening an issue or pull request.

## Before you start

- This repository contains the **Mono edition** only — the free, self-hostable, AGPL-3.0 licensed version of RMS Mail.
- Other editions (Mono Pro, Unified, Teams) are closed-source and distributed as prebuilt Docker images. Please don't open issues/PRs asking for their source — that's a deliberate business decision, not an oversight.
- Check existing issues and pull requests before opening a new one, to avoid duplicates.

## Contributor License Agreement (CLA)

Before we can merge any pull request, you'll need to agree to our [Contributor License Agreement](./CLA.md).

In short: you keep copyright over your contribution, but you grant RMS Digital Services a broad license to use it — including in the closed-source editions of RMS Mail. This lets us keep improving both the open Mono edition and the commercial editions from the same contributions, without legal ambiguity later.

When you open a pull request, a CLA Assistant bot will comment asking you to confirm. Your PR won't be merged until you do.

## Reporting bugs

Please include:
- RMS Mail version (from `CHANGELOG.md` or the Docker image tag)
- Steps to reproduce
- Expected vs. actual behavior
- Relevant logs (redact anything sensitive — email addresses, tokens, credentials)

## Suggesting features

Open an issue describing the use case, not just the feature. If it's specific to Pro/Unified/Teams functionality, it's likely out of scope for this repo.

## Submitting a pull request

1. Fork the repository and create your branch from `main`.
2. Keep changes focused — one logical change per PR is easier to review and merge.
3. Make sure `go build ./...` and `go vet ./...` pass with no errors.
4. Update `CHANGELOG.md` if your change is user-facing.
5. Open the PR and confirm the CLA when prompted.

## Code style

- Go: standard `gofmt`/`go vet` conventions. No non-standard formatting tools required.
- Frontend (Next.js/TypeScript/React): follow existing patterns in the codebase — check a neighboring component before introducing a new convention.

## Security issues

Please don't open a public issue for security vulnerabilities. Email **info@rms-ds.com** directly instead.

## Questions

Open a [Discussion](../../discussions) if available, or reach out at **info@rms-ds.com**.
