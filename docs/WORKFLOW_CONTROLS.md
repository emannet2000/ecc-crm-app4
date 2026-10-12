# Stage gates, client timelines, escalations and deal boards

## Application stage gates

Managers configure destination-stage policies under **Workspace tools → Stage gates**.
Policies begin disabled. A policy can select all required application documents,
specific document names, uploaded files, review of each current file version,
Verified status and manager approval. Selected expired documents block the move.

Each application’s record tools show **Application stage gates**: select a destination
to see unmet requirements. Managers can approve the current application and selected
document versions with a review note. Changes to those records or the policy invalidate
that approval. Approval alone does not waive document requirements.

A manager can move an application despite unmet requirements by selecting override
and entering a reason. Approvals, gated transitions and override reasons are recorded
in the application’s recent history and client timeline. Policies and rules also have
a workspace security audit. Concurrent policy edits and stage moves require the
version that was reviewed; refresh after a conflict.

Gates apply to persisted application stage changes, including existing forms and
workflow edits. Automatic document-driven advancement waits at a blocked gate while
the document edit itself is saved. Requirements apply when entering a stage; enabling
a policy does not move applications already at that stage backwards.

## Unified client timeline

Open **Client history** on a contact, or **Unified client timeline** in an application
or student’s record tools. Events include linked inquiries, applications, students,
deals, conversations and email, document uploads and reviews, stage changes,
approvals, handoffs, invoices, payments, credits, refunds and record changes.

Only explicitly linked records the signed-in staff member can access are included.
A student with multiple linked clients asks you to choose the client. Search, event
category and date range filters work across the timeline; results are paged, newest
first. Events retain their source dates and links. Historical data only appears where
it was previously recorded; the timeline does not invent older events.

## Manager escalation rules

Managers add and edit rules under **Workspace tools → Escalations**. Choose overdue
applications, lead follow-ups, tasks or unanswered application handoffs, then set a
1–365 day delay and either a specific active manager or all eligible managers.
New rules start disabled until you enable and save them.

The existing reminder worker evaluates rules every 15 seconds. Due work uses the
workspace calendar date; unanswered handoffs use elapsed time from their creation.
Recipients must be active managers in the same workspace with access to the record.
One notification is delivered per rule, record, deadline or handoff, and recipient.
Resolving the work or disabling a rule retires its unread alerts. Changing a deadline
creates a new overdue episode. Rules do not send client messages.

## Progressive deal board

Search, owner and closing-date filters run on the server. Board totals, stage counts,
conversion and currency totals include every accessible matching deal. Amounts remain
separate by currency. Owner choices come from all accessible search matches.

The board initially loads 50 cards. **Load more deals** appends another page;
**Refresh board** reloads current data. A deal change in the workspace invalidates a
paging snapshot, so the board asks you to refresh instead of mixing pages from
different versions. Bulk selection applies to selected loaded cards.

API: `GET /api/deals?board=page&limit=50&offset=0`, with optional `q`, `owner`,
`from`, `to` and `snapshot`. Responses include `summary`, `owners` and `snapshot`.
`owner=__mine__` uses the signed-in staff assignment. The older `board=all` endpoint
remains available for existing consumers; the app uses progressive paging.

## Verification

Backend coverage: `go test -run 'TestStageGate|TestProgressiveDealBoard|TestManagerEscalations|TestUnifiedClientTimeline' ./...`
from `golang-backend`. Browser coverage: `node --test tests/workflow.browser.cjs`.
