# Record cleanup and ownership

## Review old assignments

Open **Workspace tools → Team & access → Historical staff assignments**.
The list includes leads, cases, deals and tasks with a name or unavailable account
instead of an eligible active staff ID. Only accessible records appear; only
record owners and managers who can edit a record can save its mapping.

A suggestion is shown only for a unique exact name match, ignoring surrounding
spaces and letter case. Aliases such as Flor/Flordelix and duplicate staff names
need a person's review. Choose the account and confirm that it is the same person
before saving. No assignment is mapped automatically. Saved mappings preserve
record ownership, team and visibility. Stale mappings are rejected.

## Audit records

Open **Workspace tools → Data & views → Run data quality scan**. Use severity,
module and search filters, then open the finding's record for review.
The scan includes converted leads without links, unavailable conversion/application
links, matching lead/client identities, missing next actions and existing document,
student and deadline checks. Closed/Lost leads do not need follow-up dates.

Duplicates are possible matches, not instructions to merge. An unavailable link
can mean a missing record or a record outside the current user's access. Ask an
authorized manager to review it. The scan does not rewrite records or bypass access.

## Dashboard

**Needs attention** contains open work, overdue follow-ups, requirements, deadlines
and handoffs. Counts can overlap. **Record overview** contains deal, open pipeline,
lead, student and case totals across all stages in the selected assignment view.
Click any count to see its matching records and links. The duplicate totals row
has been removed; the pipeline charts remain below.

## Responsibility and record ownership

Staff IDs display as staff names in lead/case/deal details, deal cards and owner
filters, record summaries and work lists. Inactive directory accounts are labeled
inactive where a directory label is shown; unknown historical text is preserved
until reviewed. My deals uses the assigned deal owner, with record-owner fallback
for unassigned deals.

The application's **Ownership and handoffs** panel distinguishes its assigned
adviser from its record owner. Use a handoff to transfer established responsibility;
the current owner remains responsible until acceptance. Acceptance saves the
receiving account ID and transfers ownership using the existing permission checks.
An unchanged historical assignment can be mapped to the same person without a
handoff. The case editor links to the handoff panel and rejects direct changes to
a different established adviser.

## Deals layout

Deals keeps search and Add deal at the top. Staff and closing-date filters expand
on demand and are initially expanded when filters are active. All five metrics are collapsed under Pipeline metrics so the Kanban board stays
prominent. Cards omit age badges, and columns omit weighted totals and value bars. Open pipeline and expected value exclude Won and Lost, with separate
totals for each currency. The board loads all accessible matching deals from one committed SQL snapshot.
Metrics and staff/date filters cover that full set, including deals beyond the
previous 200-record limit. Superseded search responses are ignored. Ordinary API
lists remain paginated. Large boards transfer and render more data; measure browser
performance for unusually large workspaces. Phone layouts stack stages vertically. Card actions
remain available on touch devices and when a card receives keyboard focus.
