---
scenarios:
  - name: allowed-product-event-is-accepted
    description: A client reports a batch that contains only allowed event names and a valid visitor identifier.
    expected: The response is success, the accepted count matches the batch, and one product_event log record exists for each event.
    tags:
      - backend-api
  - name: unknown-product-event-is-rejected
    description: A client reports a batch that includes an event name outside the allow list.
    expected: The request is rejected as a caller error and no product_event records are written.
    tags:
      - backend-api
  - name: product-event-can-be-found-without-login
    description: An anonymous visitor reports a page view, then later reports a play event with the same visitor identifier.
    expected: Both records share that visitor identifier and can be retrieved together even though no account identity is present.
    tags:
      - backend-api
