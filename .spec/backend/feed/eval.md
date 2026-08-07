---
scenarios:
  - name: latest-feed-cursor
    description: A client can read the latest feed across two cursor pages without duplicate or missing video IDs.
    expected: Adjacent pages form a stable ordered sequence and the next cursor advances or ends the sequence.
    tags:
      - backend-api
  - name: following-feed-auth
    description: An authenticated user can read the following feed and cannot use another user's following scope.
    expected: Results contain only followed authors for the authenticated account and unauthenticated access is rejected.
    tags:
      - backend-api
  - name: popularity-snapshot
    description: A popularity feed remains stable while a client requests its next page.
    expected: The second page uses the same ranking snapshot boundary as the first page.
    tags:
      - backend-api
  - name: popularity-mysql-fallback
    description: When Redis has no usable popularity entries or cannot be read, the popularity feed returns playable videos ordered by persisted MySQL popularity.
    expected: The response is non-empty when MySQL has playable videos, returned popularity values match MySQL, and as_of is zero.
    tags:
      - backend-api
