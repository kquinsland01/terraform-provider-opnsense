# Story 31.0j: Provider Retry Settings

## Status

backlog

## Priority

P1 — complete before Stories 31.1–31.3.

## Evidence and Scope

Expose the retry controls already supported by `pkg/opnsense.ClientConfig`: maximum retries, minimum wait, and maximum wait. The pinned provider uses `retries`, `min_backoff`, and `max_backoff` with matching environment variables and positive-value validation. Preserve local defaults (3, 1 second, 30 seconds), sensitive credential handling, and the local maximum-read-concurrency safety behavior.

## Acceptance Criteria

- **Given** HCL or environment retry settings, **when** provider configuration resolves, **then** HCL takes precedence, values are validated, and exact values reach `ClientConfig`.
- **Given** omitted settings, **when** the client is created, **then** current retry defaults remain unchanged.
- **Given** transient and non-retryable responses, **when** requests run, **then** tests prove configured retry count/backoff selection without retrying permanent client errors.

## Verification

Provider/client unit tests, generated provider docs, and `make check`.
