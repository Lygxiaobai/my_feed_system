---
scenarios:
  - name: api-worker-process-boundary
    description: The API and Worker start as separate processes using the shared configuration contract.
    expected: The API serves its HTTP surface without owning Worker consumers, and the Worker consumes asynchronous work without replacing the API routes.
    tags:
      - backend-api
  - name: runtime-configuration-failure
    description: A process starts with an unreadable or invalid configuration.
    expected: Startup fails clearly instead of silently using incomplete connection or security settings.
    tags:
      - backend-api
  - name: response-envelope-is-uniform
    description: A caller invokes a successful endpoint, a rejected endpoint, and an unauthenticated endpoint.
    expected: All three answer with the same envelope shape carrying business code, user-facing message, payload, and request identifier; the success code is distinct from the failure codes; the HTTP status still reflects transport meaning.
    tags:
      - backend-api
  - name: internal-error-detail-never-reaches-caller
    description: A request fails validation and another fails inside a dependency.
    expected: Each response carries only a user-facing message and a business code, while the underlying error text is present in the log record sharing that request identifier.
    tags:
      - backend-api
  - name: request-identifier-correlates-response-and-logs
    description: A caller records the request identifier from a failed response and searches the logs for it.
    expected: The log records for that one request are found by that identifier, including both the failure record and the access record.
    tags:
      - backend-api
  - name: log-severity-matches-failure-origin
    description: The system serves a request rejected for caller error and a request that fails inside the system.
    expected: The caller error is recorded at warning severity and the system failure at error severity, so alerting on errors is not triggered by ordinary caller mistakes.
    tags:
      - backend-api
  - name: logs-are-machine-readable-and-redacted
    description: The API and Worker run through startup, a login, and a query that matches no row.
    expected: Every log record is structured and parseable with a severity and the emitting service, no credential or token value appears, no query text with inlined parameters appears, and a normal empty result is not recorded as a failure.
    tags:
      - backend-api
  - name: liveness-and-metrics-produce-no-access-logs
    description: The liveness endpoint is polled repeatedly and the metrics endpoint is scraped.
    expected: Neither produces access-log records, so log volume reflects real traffic rather than monitoring.
    tags:
      - backend-api
  - name: startup-fails-when-required-secrets-are-absent
    description: A process starts with no credential values supplied by the environment.
    expected: Startup aborts and names every missing variable in one message; no component receives an empty password or an empty signing key.
    tags:
      - backend-api
  - name: startup-rejects-a-weak-signing-key
    description: A process starts with a signing key that is present but too short to resist offline attack.
    expected: Startup aborts with a message stating the required length and how to generate a suitable key.
    tags:
      - backend-api
  - name: defaults-cover-non-secret-configuration
    description: A developer supplies only the credential values and starts the system.
    expected: Every remaining setting resolves to its declared default, and any value explicitly provided by the environment overrides both the local file and the default.
    tags:
      - backend-api
  - name: repository-contains-no-credential-values
    description: The published repository is inspected for credentials.
    expected: Configuration files hold placeholders only, the untracked local file is excluded from version control, and no credential value appears anywhere in the tree.
    tags:
      - backend-api
