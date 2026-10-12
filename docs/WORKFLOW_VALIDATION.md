# Workspace workflow validation — 2026-10-12

## Scope and evidence

The isolated local environment uses synthetic demo records plus 5,000 contacts,
applications and required documents; 20% belong to another workspace. No actual
staff participants, live provider credentials, approved recipients or deployment
backup were supplied. The staff environment question remains pending.

`npm run test:workspace-next` checks four Workspace groups, administrator/member/
viewer/manager navigation, forbidden recovery deep-link fallback, mobile overflow,
missing and overdue filters, draft protection on filter changes, saved document
responsibility/deadlines and collapsed secondary application panels. Expiry
boundary tests cover today, day 30, day 31 and already-expired documents. Existing
permission and revision tests continue to cover hidden records and stale saves.

The test stops its source server before backup, creates an archive with the
actual backup command, restores into temporary storage, verifies the manifest,
database integrity and referenced file checksums, then boots a separately restored
app. It signs in, verifies the saved document plan and downloads the restored
attachment, checking its exact content. This establishes local synthetic-workspace
restoration; a deployment backup restore remains to be scheduled separately.

Evidence is recorded in `tests/artifacts/next-workspace-report.json` and
`tests/artifacts/next-integration-assessment.json`, with the mobile screenshot in
`tests/artifacts/improvements-application-phone.png`. Artifacts are generated locally and
are excluded from source control. The browser test runs in CI as well.

Build, all five frontend suites, backend tests, Go vet, four Python backup tests
and the representative browser/restore check passed. The full backend race check
also passed. Existing improvement browser checks
also passed, along with 33 browser fixture checks and all four local live-app
checks (including the complete education journey). Live-app fixtures use local
servers and do not establish live provider acceptance.

## Live integrations and staff sessions

The integration runner performs a read-only assessment against the isolated app.
Email, WhatsApp, payments and AI are unconfigured there. No messages or payments
were sent. These results are not live provider acceptance.

Actual adviser and finance participation remains pending. New observation tasks
**workspace-navigation** and **document-followup** are available in Staff usability,
and the [session worksheet](STAFF_SESSION_WORKSHEET.md) includes their prompts,
time limits and expected saved outcomes. Use normal staff accounts and harmless
test records; record completion, elapsed time, errors and help. Turn actual
obstacles into reproducible issues and retest after fixes.

Complete provider validation with the designated environment, configured test
accounts and dedicated recipients using [integration acceptance](INTEGRATION_ACCEPTANCE.md).
Keep unperformed checks pending. Neither fixtures nor readiness indicators establish
live delivery, actual staff usability or deployment recovery.

## Obstacles found by automated review and fixed

- A flat list of tools obscured their workflow grouping and exposed recovery
  navigation to roles unable to use it. Navigation is grouped, capability-aware,
  and falls back to daily notifications for unavailable global deep links.
- Application requirements, record-tool overview and document lists repeated the
  next action and document information. Essentials stay prominent; secondary
  records, stage controls and activity are collapsed, and the record overview
  avoids repeating essentials already present on the page.
- Application overview called an unavailable client-task helper and displayed a
  load failure. The obsolete invocation is removed, and browser checks now reject
  a failed overview rather than checking only the essentials panel.
- Large Files lists rendered thousands of rows. Filters retain full accessible
  counts while rows are paginated without rebuilding the upload form.
- Priority/attention requests projected unused workspace records, and operational
  document reporting repeatedly scanned application parents. Scoped SQL snapshots
  and parent indexes reduce those costs; measurements are in
  [PERFORMANCE.md](PERFORMANCE.md).

These are automated/code-review findings. They are not attributed to staff.
