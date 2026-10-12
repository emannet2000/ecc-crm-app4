# Client work on desktop and phone

Open a contact, inquiry, student or application to work with its explicitly linked
records in one client workspace. Next actions, Files, Communication, Payments and
History keep the selected application or inquiry in view. Payments explicitly
labels a separate invoice selector and keeps the current application context. Editing a linked record opens the normal form
without leaving the client workspace. Client details and additional tools remain
available in the expandable section below.

Unconverted inquiries expose CV upload and conversion in Next actions. Linked
applications expose their next action, visible overdue follow-ups and missing-file
counts. All five work tabs remain visible on phones. Invoice and application
creation open prelinked forms directly from the workspace. Communication
uses an accessible client, application or student; invoice tools stay on the
Payments tab. Records are joined through saved IDs, never matching names or email
addresses. Each related record must independently pass workspace and visibility
checks. The server supplies its edit capability; a shared record is not necessarily
editable. The selector shows at most 100 linked records per type and explains when
more remain available through the module lists.

On phones, sign-in has visible field labels, password-manager support, a password
visibility control, comfortable touch targets, and a single column that can scroll
with the keyboard open. Home, Applications, Chat, Tasks and More share one bottom
navigation row. The Finance preset replaces Applications with Invoices in the bottom row.

## Form state and conflicts

Elm decodes each record's revision and edit capability. A form retains the revision
of the record it opened with. Updates, deletions, stage changes, task status changes,
refund requests and bulk deal changes send explicit revision information through
the API. Background record notifications refresh data without clearing open forms.
The server remains responsible for all access and version enforcement.

Elm stores typed draft fields through `Ports.draftStorage`; `public/draft-storage.js`
only reads, writes or removes those values in this tab's session storage. Recovery
keys include the user, record type and record ID; new-record drafts also include
the parent route. Existing-record drafts can be recovered from either the client
workspace or the module detail screen. Passwords, file
selections, consent and confirmation checkboxes are not restored. Successful saves
and explicit discards remove the draft. A failed save keeps entries and offers a
comparison with the latest saved fields; the original revision stays in use until
the staff member explicitly confirms the comparison. A later concurrent edit can
still reject the save.

The previous DOM-based draft scanner and global XMLHttpRequest hooks have been
removed. Record actions use typed capability checks instead of matching button
labels. DOM observers remain for presentation behavior such as keyboard focus,
responsive tables and theme propagation.

## Verification

`npm run test:client-workspace` runs an isolated browser against a temporary demo
database. It checks narrow and landscape sign-in, bottom navigation, inline record
editing, refresh during a draft, stale-save rejection, conflict comparison, duplicate
submission prevention, recovery after reload, dirty-plan protection and viewer
controls. It saves phone screenshots in `tests/artifacts/`.

Backend tests cover linkage, hidden parents, foreign workspaces, unrelated people
with matching names, read-only capabilities and stable revision hashes across roles.

## Deals

Deals invalidates and replaces in-flight requests on navigation instead of leaving
the board stuck in Loading. Refreshes retain the current cards and recover after
failures. Search remains busy through the response, and stale results are rejected.
A stage selector makes the phone board usable without horizontal scrolling; stage
counts still cover all matching records. Stages whose cards are on another page
explain that more cards must load. Cards render lazily, client links do not depend
on a previously loaded contact page, and read-only stage controls are disabled.
The server reuses the stage aggregation count and indexes organization ordering
and owner/date queries. `npm run test:deals` checks delayed navigation, refresh
failure, search completion, and stage selection against an isolated database.

The local 5,000-deal benchmark returned the first 50 scoped cards plus full-board
metrics in about 48 ms per request (three iterations on the test machine). This
is a local workload measurement, not a production latency guarantee.
