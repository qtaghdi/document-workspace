# Shared Product Specification

Status: target product contract, with implementation status recorded below.
Baseline: 2026-10-08, implementation commit `4a6cfeb`.

This specification defines observable behavior across document formats. It is
not a claim that every requirement is implemented, a new API schema, or a
commitment to reproduce every feature of desktop office applications.
[Format support](format-support.md) defines the supported scope per format.
[Architecture](architecture.md) defines ownership and implementation boundaries.
[Product plan](product-plan.md) defines delivery order.

## Users and Primary Workflows

A person opens or creates a document, inspects it, edits directly, and asks an
AI agent to make changes. Both work against the same server-owned revision.
The person can see where the agent is working and whether a change is pending,
committed, conflicted, or failed. The saved result can be reopened in its
intended application and checked against the declared compatibility scope.

The first delivery is local and single-document. Multi-document navigation,
hosted accounts, and simultaneous multi-user collaboration are later scope.
Creation, editing an existing source, and conversion are separate workflows.

## Requirement Status

- Implemented scope: available within the current XLSX implementation only.
- Partial: some behavior exists, but the complete requirement is not met.
- Planned: a requirement for future work, not an available capability.

| ID | Requirement | Current status |
| --- | --- | --- |
| DOC-01 | Open a supported source and expose its capabilities and limits | Partial: existing XLSX at process startup; no generic discovery |
| DOC-02 | Create a blank or template-based document at an explicit destination | Planned: no public creation workflow |
| EDIT-01 | Human and AI edits use one validated, revisioned commit path | Implemented scope: supported XLSX operations |
| EDIT-02 | Show AI location, intent, and commit progress | Partial: XLSX presence and events; host-dependent presentation |
| EDIT-03 | Reject stale writes without partial mutation | Implemented scope: XLSX base-revision checks |
| SAVE-01 | Persist accepted batches safely and report the durable result | Implemented scope: atomic local XLSX replacement |
| SAVE-02 | Save a copy, export, or convert with explicit destination and limits | Planned: no general export or conversion workflow |
| HIST-01 | Restore available history as a new revision | Implemented scope: bounded local XLSX undo and redo |
| AUTH-01 | Respect host tool permissions and server access controls | Partial: host-managed approvals and local tokens; no shared approval store |
| AUTH-02 | Review a proposed change before committing when requested | Planned: no generic staged proposal or review UI |
| COMP-01 | Report preservation, rendering, and editing limits separately | Partial: XLSX notices and corpus; no complete capability report |
| HOST-01 | Work through MCP, with an embedded editor or browser fallback | Partial: transports implemented; certification varies by host and mode |
| REC-01 | Recover safely from disconnects, retries, and external file changes | Partial: event replay and history checks; full recovery contract pending |

## Document Lifecycle

### Opening and Creation

DOC-01 requires validating the format and file limits before opening a writable
session. Unsupported, corrupt, encrypted, or otherwise restricted inputs must
produce an actionable result without modifying the source. An extension alone
must not establish compatibility. A session must identify its document, format,
revision, supported operations, and detected limitations.

DOC-02 requires an explicit format and destination. Blank creation and creation
from a template must be tested separately. An existing destination must never
be silently replaced. A template remains unchanged. A newly created document
must reopen successfully in the target application before creation is declared
supported. Internal fixture generators do not establish product support.

### Editing and Persistence

EDIT-01 and SAVE-01 require every batch to identify its base revision and actor.
The server validates the complete batch, applies it as one logical transaction,
persists it, advances the revision once, and publishes ordered commit events.
An invalid operation rejects the batch. A persistence failure must not produce
a success response or a committed UI state.

The current supported edit workflow saves each accepted batch automatically.
Explicit review is a future mode, not an additional confirmation on every
ordinary edit. A pending animation or browser draft is not a saved change.
Closing an editor must distinguish committed work from unsent drafts.

SAVE-02 distinguishes saving the native format, saving a separate copy, and
exporting another format. Each conversion is a directed capability, such as
DOCX to PDF. Supporting both endpoints does not imply a converter between them.
Outputs must include relevant fidelity limits and retain the original source.
A failed export must not leave a partial file presented as a successful output.

## Collaboration and Recovery

EDIT-02 requires an actor label and a format-specific location: a cell range,
text selection, slide object, or PDF page region. Cursor and typing events are
ephemeral and must not advance the document revision. Reduced-motion settings
must suppress unnecessary animation without hiding commit status.

EDIT-03 requires stale edits to return a conflict and the current revision.
The client must refresh and deliberately reapply or revise the intended edit.
It must not overwrite newer human work through an automatic blind retry.
Real-time visibility does not imply character-level collaborative merging.

HIST-01 restores document content as a new revision rather than moving the
revision counter backward. History limits and unavailable restoration must be
visible. Current XLSX history is bounded to ten snapshots, with snapshots over
32 MB excluded; it is not an unlimited backup or audit archive.

REC-01 requires resuming ordered events when possible and loading an
authoritative snapshot when a replay gap cannot be filled. A lost response
after persistence must be resolved before retrying a write. A future retry
contract must distinguish already-committed operations from new operations.
External file replacement must be detected before overwriting newer external
content. End-to-end retry and external-write protection remain release work;
existing history hash checks alone do not satisfy this requirement.

## Permissions and Review

AUTH-01 distinguishes host permission from document validation. Claude and
Codex own their tool approval controls; the service must not promise identical
permission labels or durations across hosts. Authentication and authorization
must still be checked on the server. Permission to call a tool does not bypass
revision checks or authorize unrelated paths or destructive conversion.

AUTH-02 applies when a user asks to review changes before application. A future
proposal must record its document, base revision, operations, and affected
content. Approval authorizes that proposal, not future unrelated changes.
Rejection leaves the document unchanged. A newer revision invalidates the
proposal until it is refreshed and reviewed again. Until this workflow exists,
the product must not advertise staged approval or infer it from host approval.
See [host compatibility](host-compatibility.md) for current host behavior.

## Compatibility and Host Experience

COMP-01 evaluates source preservation, visual rendering, editable features,
calculation, and conversion independently. Unsupported content must remain
intact during unrelated edits when preservation is claimed. Known destructive
edits must be blocked or offered as an explicitly accepted derived copy.
Unknown fidelity must be identified as unverified, not described as lossless.

HOST-01 requires useful structured MCP results even without an embedded editor.
A supported host may show the MCP App; another may use the authenticated local
browser fallback. Certification must record host version, connection mode, and
the workflow tested. Passing a protocol test does not certify embedded UI.
The editor must support keyboard access and a usable small-screen fallback.

Document contents are untrusted data. Opening a file must not execute macros,
scripts, embedded programs, or arbitrary external resource requests. Source
contents and credentials must not appear in routine logs. Format-specific
resource limits must be defined and exercised before adapter release.

## Acceptance Scenarios

These are release criteria, not claims about tests already implemented.

| Scenario | Required observable result |
| --- | --- |
| Open an unsupported or malformed file | Actionable error; source bytes unchanged |
| Create a document or use a template | New native file reopens; template and existing destinations remain intact |
| Commit a human edit, then an AI batch | Both surfaces converge on the same revision and saved content |
| Submit two writes at the same base revision | One commits; stale write conflicts without partial changes |
| Submit a batch containing one invalid edit | Entire batch fails; content and revision remain unchanged |
| Fail persistence | No committed success; last durable document remains recoverable |
| Disconnect during a commit | Reconcile durable state before retry; no duplicate edit |
| Replace the source externally | Detect conflict before an overwrite |
| Undo, redo, and reopen | Restored content persists; revisions increase; limits are reported |
| Reject or approve a staged proposal | Rejection changes nothing; approval applies only a current reviewed proposal |
| Edit beside an unsupported feature | Claimed preserved content survives save and reopen |
| Open through a host without embedded UI | Structured tools remain usable; supported fallback reaches the same session |
| Export or convert | Separate valid output; original retained; limitations disclosed |

## Delivery Boundaries

The XLSX stable milestone uses the applicable implemented subset, with explicit
exclusions for creation, conversion, generic proposals, and other pending
capabilities. Its release checklist must resolve or explicitly bound remaining
recovery gaps. It must not be presented as completion of this entire spec.

DOCX is the second adapter. Its first prototype must prove an unrelated edit
can survive native save and reopen before selecting a broad editing surface.
Only then should shared runtime contracts be extracted from both adapters.
No additional runtime framework or paid editor dependency is selected here.
