# context and compaction

wraith builds each chat request from its cybersecurity system prompt, global user instructions, exact pins, the compacted working state, the fact ledger, and current conversation messages. reasoning returned by the model is stored for display and recovery but is not replayed as conversation content.

`/remember` and `/instructions` apply across chats. `/pin` is for exact details that must remain in this chat's active context: targets, constraints, identifiers, paths or decisions. `/context` reports estimates and the last token count measured by ollama. the header's `~` marks an estimate, not an exact tokenizer count.

## budgeting

before sending, the app estimates input tokens and reserves the configured output budget plus a small margin. it scales estimates upward when ollama's last measured prompt exceeds the corresponding estimate. an oversized request is stopped before transmission and the draft is retained. the app does not discard old messages to make it fit. tokenization is model-specific, so estimates cannot guarantee that the server will never truncate; use a margin and inspect measured counts.

threshold checks happen after complete assistant responses. with `compact.mode=auto`, compaction starts when usage reaches the configured threshold, 90% by default. with `ask`, a popup asks after each response above that threshold. `n`, the no option, or closing that threshold popup counts as a decline. the third decline starts compaction. successful compaction resets the counter.

`manual` disables threshold prompts and automatic compaction. `/compact` remains available. a draft that cannot fit has a separate pre-send compact prompt in ask/manual modes; declining it retains the draft and does not send anything. auto mode tries compaction directly. output reservation can require this earlier than the 90% threshold.

## replacement transaction

1. copy the complete current session into the gzip recovery cache.
2. write a separate immutable gzip archive before requesting any summary.
3. retain the configured recent user/assistant pairs verbatim.
4. divide older messages into bounded, unicode-safe fragments with message, role and part labels.
5. send each fragment with a dedicated extraction prompt requesting structured working state and durable facts. the normal chat instructions and unrelated history are not dumped into this job.
6. send the source fragment and candidate memory through a separate omission audit. append missing facts returned by that audit.
7. preserve prior ledger entries, pins and selected exact literals from the source. keep previous working state in the ledger.
8. reject empty, malformed, truncated, oversized or non-shrinking replacements. otherwise atomically save the candidate session, then replace the in-memory state.

extraction uses native thinking off when supported, otherwise low when supported, otherwise the model default. each extraction and audit has its own output allowance; they share the operation timeout. cancellation or failure keeps the original session active and leaves the archive available. a draft edited during compaction remains a draft for review.

## preservation limits

the full archived transcript is lossless, and pins plus existing ledger strings are retained exactly. model-written summaries and audits cannot prove that every meaningful detail survived. pin critical literals, review the compacted state with `/export`, and use `/archives` whenever the original wording matters.

the audit is a separate request to the selected model, not a different independent model. deterministic literal recovery covers backtick spans, urls, ipv4/port strings, paths and cve identifiers; it is not a universal parser for every identifier or source language.

protected facts are never silently pruned. a large ledger may eventually prevent useful compaction. the app then retains the original and reports the limit. increase context if hardware permits, reduce retained recent turns or start a new chat carrying the pins you choose.

## recovery

`/archives` lists originals for the current chat. selecting one creates a new session id and keeps the archived file unchanged. `/restore filename.json.gz` does the same explicitly. the recovery cache is an additional last-known original; the permanent archive directory retains earlier originals as well.

active sessions, archives and recovery snapshots all contain the session schema in compressed json. inspect a copy with `gzip -dc file.json.gz`. edits to assistant replies also archive the previous session before replacement. failed compaction attempts may leave valid archives that were never referenced by a committed replacement; the archive picker includes them.
