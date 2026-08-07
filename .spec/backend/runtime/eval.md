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
