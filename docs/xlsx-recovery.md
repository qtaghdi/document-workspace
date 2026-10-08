# XLSX Recovery Operations

## Save Failure or Disconnection

The browser pauses further writes and history actions when a save response is
lost, a save fails, or live synchronization breaks. It reads the current server
revision when possible and shows a recovery panel. The server revision alone
does not prove whether a particular write committed. No write is automatically
retried with a newer base revision.

Copy any unsaved draft content before choosing **Reload saved workbook**. Reload
discards the displayed drafts and reads server state. Check the affected cells
or objects before repeating an uncertain edit. Drafts are not durably backed up.
If the server cannot be reached, restore the connection first. A failed MCP
write requires the same read-and-inspect process in the host.

## External Source Changes

If another application changes or replaces the XLSX file, apply and undo/redo
reject the write. The existing session retains its last authoritative in-memory
state; refreshing the browser alone does not reopen the external file.
Stop the document-workspace process, retain any needed drafts, inspect the
external source, and restart the process against the intended file. Do not
restore an older source merely to make a conflict disappear.

The service serializes its own writers with a sidecar lock and checks source
hashes before mutation and again before rename. This is not a filesystem
compare-and-swap primitive. Excel, hard-link aliases, and other applications
can race the final check. Use one writer for a file at a time. Network storage
and simultaneous native-application editing are not certified by these checks.

## Interrupted Writer Lock

The lock is an empty `write.lock` directory beneath the workbook-specific
directory in `.xlsx-viewer-history`. A process crash can leave it behind.
Opening or writing fails closed rather than stealing a potentially active lock.

Before removing a stale lock, stop all document-workspace processes using that
workbook and confirm that no writer is active. Back up the workbook and its
history directory. Identify the exact lock for that workbook, then remove only
that empty lock directory. Never delete the whole history store or pending
manifest as a shortcut. Restart the service so its pending-manifest recovery
can compare the saved workbook hash and finalize or discard the pending entry.
If lock ownership is uncertain, leave it in place and investigate first.

## Verification and Remaining Limits

`recovery_test.go` in the XLSX adapter covers temporary creation, write, short
write, sync, close, rename, and history preparation failures for edits and undo;
external replacement; a late external change; competing sessions; slow
subscribers; replay gaps; and pending-manifest recovery. Transport tests cover
HTTP conflict and SSE session mismatch. Frontend tests exercise lost responses,
blocked queues, reconnect failure, resync, and undo sequencing.

Run `make compatibility-test` and `make verify` for changes to this behavior.
The generated and LibreOffice fixtures provide bounded fidelity evidence;
Microsoft Excel-produced fixtures and native desktop checks remain separate.
These tests do not certify power-loss durability, network filesystems, an
unlimited audit log, or durable per-operation receipts across process restarts.
A pending history-finalization failure blocks subsequent writes until recovery
succeeds, even if the previous workbook save itself completed.
