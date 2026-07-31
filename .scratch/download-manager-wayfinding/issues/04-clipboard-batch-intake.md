Status: Closed
Labels: wayfinder:grilling
Parent: ../spec.md
Assignee: opencode

## Question

How should clipboard batch intake work in the MVP? Define when clipboard content is observed, URL parsing and validation, duplicate behavior, confirmation versus automatic enqueueing, destination selection, feedback for accepted/rejected links, and behavior for mixed text containing multiple HTTP/HTTPS URLs.

## Comments

### Resolution

Inspect the clipboard only after an explicit user action; do not monitor clipboard changes in the background. Extract every absolute `http://` or `https://` URL from mixed text, trimming surrounding whitespace and rejecting malformed or credential-bearing URLs. Deduplicate within the batch and skip URLs already represented by any queued, active, paused, failed, or complete item, reporting skipped duplicates.

Show a review step before enqueueing, including the accepted URL list and rejected/skipped counts. On confirmation, enqueue the accepted URLs using the configured default download directory for the whole batch. Show a categorized result summary with accepted, skipped-duplicate, invalid/rejected, and failed-to-enqueue counts, expandable URL details, and a link to the queued items.
