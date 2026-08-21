---
scenarios:
  - name: reporting-does-not-change-visibility
    description: Several distinct accounts report the same video, then every public listing and the detail route are checked.
    expected: The video remains exactly as visible as before, in every listing and to every viewer, no matter how many reports accumulate.
    tags:
      - backend-api
  - name: reporting-channel-survives-moderation-being-off
    description: Automated moderation is disabled and a viewer reports a video.
    expected: The report is accepted and enters the review queue, so the notice channel does not depend on the optional moderation capability.
    tags:
      - backend-api
  - name: one-report-per-account-per-item
    description: The same account reports one video twice, then a different account reports the same video, then the first account reports a different video.
    expected: Only the repeated submission is refused as a duplicate; the other two are accepted.
    tags:
      - backend-api
  - name: reporting-cannot-probe-hidden-content
    description: An account reports a video that does not exist, and separately one that exists but is not visible to it.
    expected: Both answer identically as though the content does not exist, revealing nothing about which case occurred.
    tags:
      - backend-api
  - name: self-reporting-is-refused
    description: An author reports their own video.
    expected: The submission is refused and no report is recorded.
    tags:
      - backend-api
  - name: unexplained-catch-all-reason-is-refused
    description: A reporter submits the catch-all reason with no explanation, then resubmits with one.
    expected: The first is refused for missing an explanation; the second is accepted.
    tags:
      - backend-api
  - name: reporter-sees-the-outcome
    description: A reporter files a report, a reviewer decides it, and the reporter lists their own reports.
    expected: The reporter sees their own report with its resolved outcome and decision time, sees no other account's reports, and sees neither the reviewer's identity nor the internal note.
    tags:
      - backend-api
  - name: queue-is-restricted-and-grouped-by-item
    description: A non-reviewer requests the queue; then a reviewer requests it after one video received three reports under two reasons and another received one.
    expected: The non-reviewer is refused; the reviewer sees one entry per video with its report count and reason distribution, most-reported first.
    tags:
      - backend-api
  - name: removal-hides-content-and-records-why
    description: A reviewer removes a reported video, then a non-author and the author each request it.
    expected: The video leaves every public surface including cached reads, the non-author sees it as missing while the author still sees it as rejected, and the decision, its operator, and its cause are recorded in the durable trail.
    tags:
      - backend-api
  - name: dismissal-leaves-content-untouched
    description: A reviewer dismisses the reports on a video.
    expected: The video stays publicly visible and the reports are closed as dismissed.
    tags:
      - backend-api
  - name: failed-removal-keeps-notices-outstanding
    description: A reviewer removes a video and the removal fails.
    expected: The reports stay outstanding and the item remains in the queue, so the case cannot be lost while the content is still reachable.
    tags:
      - backend-api
  - name: an-item-is-decided-once
    description: A reviewer decides an item, then attempts to decide it again.
    expected: The second attempt is refused because nothing is outstanding, and the recorded outcome of the first decision is unchanged.
    tags:
      - backend-api
