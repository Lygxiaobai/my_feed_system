---
scenarios:
  - name: comment-tree
    description: A user can publish a root comment and a reply, list them, and delete an owned comment according to the API rules.
    expected: The response preserves root and parent relationships and deletion does not leave an invalid reply tree.
    tags:
      - backend-api
