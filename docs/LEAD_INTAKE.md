# Lead intake from a client CV

**Leads** appears directly below **My work today** in every sidebar navigation
focus. On a phone, open the navigation menu first. Prospective clients start as
leads; a CV alone does not require opening an application.

## Manual intake

1. Search Leads for the client's email or name before creating another record.
2. Choose **Add lead**, enter contact details and interests, choose the source
   (for example **Referral**), and record **Referrer / referred by** (for example
   Flor), the assigned consultant and follow-up date. Save the lead.
3. Open the lead and choose **CV & conversion** or its **Files** tab. Upload the
   original CV. No case/application is required. Upload again to retain a new
   version of **Client CV**. **Versions** lists authenticated downloads.
4. Ask the referring staff member to review the details against the original CV.
   Use **Edit** to correct the lead. Put education, employment and qualifications
   in Notes when no dedicated field exists.

Uploads are private and accept files up to 10 MB. The CV picker offers PDF, DOCX,
TXT, PNG and JPEG. Lead documents inherit the lead's owner and visibility;
downloads require access to both the document and its current parent.

## Reviewed AI fields

If the workspace has activated AI and configured its provider, choose
**Prepare CV fields with AI** beside the uploaded document. This sends that CV
to the configured provider and queues a draft. Extraction supports PDF, DOCX,
PNG, JPEG, TXT and CSV up to 8 MB. DOCX text is extracted locally before being
sent; PDF and images use the existing provider document inputs.

Review the suggested name, email, phone, nationality, current country and notes.
Unknown suggestions retain existing values. The review shows missing information
and uncertainties; suggested CV notes are appended to existing inquiry notes.
Correct any field, check **I checked these fields against the original CV**, then
choose **Save reviewed fields**. Extraction alone never updates the lead.
Source, referrer, assignment, status and follow-up are preserved.

Queued and completed requests are available under **Your CV drafts** after
Refresh. If the lead changes while the review is open, reload before saving. If
the CV changes, generate a new draft. Disabled AI, unavailable providers and
unreadable drafts leave manual entry available. Automated tests use a simulated
provider; live provider acceptance remains pending.

## Convert when the client proceeds

Review the intake and choose a **Client next step**:

- **Client and application** creates or reuses a contact and opens an application
  in Assessment.
- **Student and application** also creates a student in Planning and links that
  student to the application.
- **Contact only** retains the CV on the lead. The lead can later be converted
  into an application.

Confirm the client is proceeding, then choose **Convert lead**. Links open the
resulting contact, student and application. Matching contacts are reused by
email without overwriting their existing details. Source, referrer, CV notes,
interests and follow-up context remain in the new application/student notes;
the application carries the assigned consultant and follow-up deadline.

Application conversion moves lead documents to the new application without
changing document IDs, original files or version history. Upload further versions
on the application. Repeating conversion returns its existing links rather than
creating duplicate records. Restricted matching contacts or attachments require
an authorized staff member to resolve the conversion. Converting an existing
application into a student application is not automatic; link the student from
the application tools instead.

## Verification

Backend regression checks cover upload failure rollback, original CV downloads,
version history, repeated conversion, workspace and parent visibility, staff edit
permissions, restricted attachment handling, confirmation and stale CV reviews,
and deletion/restore. The AI check uses a simulated provider with a DOCX fixture.
Chromium checks cover editable CV drafts and explicit confirmation, plus live
lead creation, referrer details, navigation presets, phone layout, CV upload and
student/application conversion against a temporary local database. These checks
do not represent actual staff sessions or live provider acceptance.

## Client intake summary

The lead CV panel shows client name, status, consultant, original creation date,
next action and follow-up date together. Its checklist reflects uploaded documents,
assignment, a scheduled action and conversion links; it does not certify CV review
or introduce additional conversion requirements. The lead detail summary is labeled
Inquiry tracking, while Convert this lead remains the explicit reviewed action.
