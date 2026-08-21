---
title: report
status: active
code:
  - backend/internal/report/service.go
related:
  - backend/internal/report/entity.go
  - backend/internal/report/handler.go
  - backend/internal/report/repo.go
  - backend/internal/video/service.go
  - backend/internal/audit/entity.go
---
# report

## raw source
Signed-in viewers can report a video they can see. Reports are recorded for human review and never change what is visible on their own.

## expanded spec
Reporting is a notice channel, not an enforcement mechanism. Submitting a report records the notice and does nothing else: it does not hide, demote, or otherwise alter the reported content, and no accumulation of reports changes that. Enforcement is a separate, deliberate act by a reviewer. This ordering is the whole point of the capability — automatic suppression driven by report volume hands the power to remove a creator's work to anyone who can assemble enough accounts, and a wrongful removal costs more than the extra minutes a genuine violation stays reachable.

The reporting channel stays available regardless of whether automated moderation is configured. Automated review is an optional capability; the ability to receive notices about content is not, and losing it would leave no path for a viewer to raise a problem.

A report is attributable and singular. Only a signed-in account may report, and one account holds at most one report per item; a second submission for the same item is refused as a duplicate rather than silently recorded. Uniqueness is enforced by the store rather than by checking before writing, so concurrent submissions cannot both succeed. A reporter may only report content that is already visible to them, so the endpoint cannot be used to probe whether a hidden or nonexistent item exists. Nobody reports their own content.

Every report carries a reason drawn from a fixed set, because free text cannot be aggregated and cannot be validated at submission. A reason that carries no inherent meaning requires the reporter to explain it, since a reviewer cannot act on an unexplained one. Supplementary text is bounded.

A reporter can see the reports they filed and the outcome of each. A notice that disappears without any observable response gives the reporter no way to tell whether the platform acted, which degrades the channel into a suggestion box. The outcome is visible; the reviewer's identity and internal reasoning are not, because disclosing them exposes both the reviewer and the enforcement threshold.

Review is organized around the reported item, not the individual notice. A reviewer sees the outstanding reports grouped by item with their reason distribution and volume, ordered so the most-reported items surface first, and decides once for that item. Deciding per notice would be both slower and capable of producing contradictory outcomes for the same content. Only reviewers may see the queue or decide; the reviewer set is the same one that governs manual content review, because the only distinction the system draws is whether an account may review.

A decision either dismisses the notices or removes the content. Removal transitions the content into the same rejected state that a failed review produces and records the decision, its operator, and its cause in the same durable trail, because a removal must remain explainable long after the logs for it have rotated away. Removal also invalidates every cached copy of the content; a state change that leaves a cached copy readable has not actually removed anything.

Removal happens before the notices are closed. If removal fails, the notices stay outstanding and the item remains in the queue for another attempt. The opposite order would mark the notices handled while the content is still reachable, producing a case that no reviewer will ever see again. Notices already closed by an earlier decision are never rewritten by a later one, so the trail cannot be retroactively altered.

## change rules
Any change that lets a report alter content visibility without a human decision contradicts this contract and requires updating this spec first. Changing the reviewer set, the reason set, or the enforcement action requires checking `video/spec.md`, because removal shares the content state machine and audit trail defined there.
