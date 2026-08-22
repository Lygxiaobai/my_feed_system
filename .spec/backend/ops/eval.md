---
scenarios:
  - name: reviewer-can-read-ops
    description: A signed-in reviewer asks for ops access and then queries logs.
    expected: Access is granted, a log query returns a bounded list or an empty list, and Grafana credentials are not present in the response.
    tags:
      - backend-api
  - name: test-email-cannot-read-ops
    description: A session created from a digits-only test-domain email that is not a reviewer asks for ops access or logs.
    expected: Access is denied and no log lines are returned.
    tags:
      - backend-api
  - name: ordinary-account-cannot-read-ops
    description: A password-only account or an ordinary email account asks for ops access or logs.
    expected: Access is denied and no log lines are returned.
    tags:
      - backend-api
