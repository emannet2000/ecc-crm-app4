# Mobile UI, team chat and management reports

## Delivered behavior

The phone navigation has five destinations: Home, Applications (Invoices for
finance focus), Chat, Tasks and More. The drawer retains access to other modules.
Phone sign-in uses a compact brand header and a full-width primary action;
authenticator/recovery codes expand on request or when sign-in reports a code
error. Email and password inputs retain password-manager autocomplete and add
mobile keyboard hints.

Chat is private staff-to-staff messaging inside the current workspace, available
at `/chat`. Active members, including viewers, can send text messages. No
administrator override exposes other people's conversations. Messages are stored
in SQLite and included in the existing database backups. Unread counts appear
in the colleague list, which supports search and pagination. Message history
loads 100 messages at a time. Active conversations poll every five seconds while
the document is visible. Sending uses an idempotent client identifier; network
failures retain the draft for retry. Drafts and pending send identifiers survive navigation and reloads in session
storage, scoped to the current user and colleague. Text typed while another
message is sending remains a draft. Sent indicates server acceptance; Read
reflects the recipient’s recorded read status. History reading preserves scroll
position, and a retry button appears when messages cannot load. Polls do not
overlap for the same request, stop when disconnected, and resume when visible
or back online. Requests time out and refresh expired sessions before retrying.
The interface includes colleague previews, date separators, keyboard sending
on desktop, and a composer fitted above phone navigation and the keyboard. This release does not add client
chat, group chat, attachments or push notifications.

Reports at `/reports` present a timestamped account-scoped snapshot, financial
position, original-currency commercial stage totals, application stages and
period activity. Date filters apply only to new-record activity, not current
balances or stage totals. Missing exchange rates are highlighted and prevent
misleading consolidated figures. The CSV includes the snapshot, scope, period,
currency breakdowns and methodology, with quoted fields and spreadsheet-formula
protection. Print / Save PDF opens a standalone A4 report through the browser's
print dialog. Secondary outcomes, analysis, record exports and audit history
remain available in expandable sections. A failed refresh retains the previous
snapshot and disables exports until a successful refresh.

Currency conversion uses configured workspace rates; it does not fetch live
market rates. Won deal value is not represented as cash collected or recognized
revenue. Report permissions remain the existing backend record permissions.

## Verification

- `./build.sh`
- `npm test`
- `cd golang-backend && go test ./... -timeout 2m`
- `go test -race -run 'TestProfessionalReport|TestTeamChat|TestReportSnapshot|TestReportsUseAllRecords' -timeout 2m`
- `npm run test:mobile-chat`: phone sign-in, five destinations, two authenticated
  users, private messages, unread counts, failed-send retry, in-app draft
  retention, older history and composer placement above mobile navigation.
- `npm run test:reports`: mixed currencies, activity periods, invalid-date
  feedback, actual CSV download, actual tagged A4 PDF creation, refresh failure
  and recovery, and 320/390/768/1440-pixel layouts.

Browser fixtures use temporary databases, synthetic users and disabled external
providers. These checks do not represent physical-device keyboard testing,
production deployment or actual staff acceptance sessions.
