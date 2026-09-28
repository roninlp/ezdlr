import {
  For,
  Show,
  createMemo,
  createSignal,
  onCleanup,
  onMount,
} from "solid-js";
import { render } from "solid-js/web";
import { createStore, produce, reconcile } from "solid-js/store";
import { Events } from "@wailsio/runtime";
import {
  AddURL,
  CancelBatch,
  CancelClipboardReview,
  ClearCompleted,
  ConfirmClipboard,
  CreateQueue,
  Delete,
  DeleteBatch,
  DeleteQueue,
  MoveToQueue,
  OpenDirectory,
  OpenFile,
  PauseBatch,
  ReadClipboard,
  RemoveBatch,
  RenameQueue,
  ResumeBatch,
  RetryBatch,
  ReviewClipboard,
  Snapshot,
} from "../bindings/ezdlr/app";
import type {
  BatchResult,
  ClipboardReview as GeneratedClipboardReview,
} from "../bindings/ezdlr/models";
import { ContextMenu, type MenuEntry, type MenuState } from "./context-menu";
import { DownloadRow } from "./download-row";
import { QueueStrip, type QueueCounts } from "./queue-strip";
import {
  ClipboardQueueID,
  EventQueueProgress,
  EventQueueSnapshot,
  Icon,
  MainQueueID,
  asState,
  formatBytes,
  padded,
  snapshotItems,
  snapshotQueues,
  type DownloadItem,
  type DownloadQueue,
  type DownloadState,
  type ItemProgress,
  type ServiceSnapshot,
} from "./ui";
import "./index.css";

type ClipboardReview = Omit<
  GeneratedClipboardReview,
  "accepted" | "duplicates" | "rejected"
> & {
  accepted: NonNullable<GeneratedClipboardReview["accepted"]>;
  duplicates: NonNullable<GeneratedClipboardReview["duplicates"]>;
  rejected: NonNullable<GeneratedClipboardReview["rejected"]>;
};

function App() {
  const [items, setItems] = createStore<DownloadItem[]>([]);
  const [queues, setQueues] = createStore<DownloadQueue[]>([]);
  const [directory, setDirectory] = createSignal("Downloads");
  const [connections, setConnections] = createSignal(4);
  const [activeLimit, setActiveLimit] = createSignal(3);
  const [url, setUrl] = createSignal("");
  const [intakeQueue, setIntakeQueue] = createSignal(MainQueueID);
  const [query, setQuery] = createSignal("");
  const [filter, setFilter] = createSignal<"all" | DownloadState>("all");
  const [selectedQueue, setSelectedQueue] = createSignal("all");
  const [message, setMessage] = createSignal("Ready when you are.");
  const [busy, setBusy] = createSignal(false);
  const [review, setReview] = createSignal<ClipboardReview | null>(null);
  const [reviewQueue, setReviewQueue] = createSignal(ClipboardQueueID);
  const [selection, setSelection] = createSignal<string[]>([]);
  const [anchor, setAnchor] = createSignal<string | null>(null);
  const [menu, setMenu] = createSignal<MenuState | null>(null);

  const filtered = createMemo(() => {
    const scope = selectedQueue();
    const visible = items.filter(
      (item) =>
        (scope === "all" || item.queueId === scope) &&
        (filter() === "all" || item.state === filter()) &&
        `${item.url} ${item.destination}`
          .toLowerCase()
          .includes(query().toLowerCase()),
    );

    if (filter() !== "all") return visible;
    return [...visible].sort((left, right) => {
      if (left.state === right.state) return 0;
      return left.state === "active" ? -1 : right.state === "active" ? 1 : 0;
    });
  });
  const activeCount = createMemo(
    () => items.filter((item) => item.state === "active").length,
  );
  const finishedCount = createMemo(
    () => items.filter((item) => item.state === "complete").length,
  );
  const totalBytes = createMemo(() =>
    items.reduce((sum, item) => sum + item.totalBytes, 0),
  );
  const totalSpeed = createMemo(() =>
    items
      .filter((item) => item.state === "active")
      .reduce((sum, item) => sum + item.downloadSpeed, 0),
  );
  const queueCounts = createMemo(() => {
    const counts: QueueCounts = {};
    for (const queue of queues) {
      counts[queue.id] = { total: 0, active: 0 };
    }
    for (const item of items) {
      const bucket = counts[item.queueId];
      if (!bucket) continue;
      bucket.total++;
      if (item.state === "active") bucket.active++;
    }
    return counts;
  });

  const queueName = (id: string) =>
    queues.find((queue) => queue.id === id)?.name ?? "Main";

  function applySnapshot(snapshot: ServiceSnapshot | null | undefined) {
    setItems(reconcile(snapshotItems(snapshot), { key: "id" }));
    setQueues(reconcile(snapshotQueues(snapshot), { key: "id" }));
    setDirectory(snapshot?.configuration?.downloadDirectory || "Downloads");
    if (snapshot?.configuration?.connections) {
      setConnections(snapshot.configuration.connections);
    }
    if (snapshot?.configuration?.activeLimit) {
      setActiveLimit(snapshot.configuration.activeLimit);
    }
    const scope = selectedQueue();
    if (scope !== "all" && !snapshotQueues(snapshot).some((q) => q.id === scope)) {
      setSelectedQueue("all");
    }
  }

  // Progress-only updates patch the rows that moved. No layout work, no IPC
  // round trip, and the shape of the list is untouched.
  function applyProgress(updates: ItemProgress[] | null | undefined) {
    if (!updates?.length) return;
    setItems(
      produce((draft) => {
        for (const update of updates) {
          const item = draft.find((entry) => entry.id === update.id);
          if (!item) continue;
          item.state = asState(update.state);
          item.totalBytes = update.totalBytes;
          item.completedBytes = update.completedBytes;
          item.downloadSpeed = update.downloadSpeed;
          item.attempts = update.attempts;
        }
      }),
    );
  }

  async function refresh() {
    try {
      applySnapshot(await Snapshot());
    } catch (error) {
      setMessage(
        `Unable to load queue: ${error instanceof Error ? error.message : String(error)}`,
      );
    }
  }

  async function add(event: Event) {
    event.preventDefault();
    if (!url().trim()) return;
    setBusy(true);
    setMessage("Adding link to queue…");
    try {
      const destination = intakeQueue();
      await AddURL(url(), destination);
      setUrl("");
      const queue = queues.find((entry) => entry.id === destination);
      setMessage(
        queue && !queue.running
          ? `Added to ${queue.name}. It waits until you start that queue.`
          : `Added to ${queue?.name ?? "Main"}. It starts when the queue has room.`,
      );
      await refresh();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }

  async function clipboard() {
    setBusy(true);
    setMessage("Reading clipboard…");
    try {
      const text = await ReadClipboard();
      const result = await ReviewClipboard(text);
      setReview({
        accepted: result.accepted ?? [],
        duplicates: result.duplicates ?? [],
        rejected: result.rejected ?? [],
      });
      // Clipboard batches default to the Clipboard queue: it exists for them
      // and starts stopped, so confirming one never begins downloading on its
      // own even when another queue has been stopped in the meantime.
      const clipboardQueue =
        queues.find((queue) => queue.id === ClipboardQueueID) ??
        queues.find((queue) => !queue.running);
      setReviewQueue(clipboardQueue?.id ?? MainQueueID);
      setMessage("Review the links below before adding them.");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }

  async function confirm() {
    const current = review();
    if (!current) return;
    const destination = reviewQueue();
    setBusy(true);
    try {
      const result = await ConfirmClipboard(current, destination);
      const added = (result.results ?? []).filter(
        (item) => item.status === "accepted",
      ).length;
      setReview(null);
      const queue = queues.find((entry) => entry.id === destination);
      setMessage(
        queue && !queue.running
          ? `${added} link${added === 1 ? "" : "s"} added to ${queue.name}. Start the queue to begin.`
          : `${added} link${added === 1 ? "" : "s"} added to ${queue?.name ?? "the queue"}.`,
      );
      await refresh();
    } catch (error) {
      setMessage(String(error));
    } finally {
      setBusy(false);
    }
  }

  async function action(task: () => Promise<void>, success: string) {
    setBusy(true);
    try {
      await task();
      setMessage(success);
      await refresh();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }

  async function bulk(run: (ids: string[]) => Promise<BatchResult>, success: string) {
    const ids = selection();
    if (!ids.length) return;
    setBusy(true);
    try {
      const result = await run(ids);
      const failures = result.failures?.length ?? 0;
      setMessage(
        failures > 0
          ? `${success} — ${failures} of ${ids.length} could not be updated.`
          : success,
      );
      setSelection([]);
      await refresh();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }

  async function createQueue(name: string): Promise<boolean> {
    try {
      const queue = await CreateQueue(name);
      setMessage(`${queue.name} created. It is stopped until you start it.`);
      await refresh();
      return true;
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error));
      return false;
    }
  }

  async function renameQueue(queue: DownloadQueue, name: string): Promise<boolean> {
    try {
      await RenameQueue(queue.id, name);
      setMessage(`Queue renamed to ${name}.`);
      await refresh();
      return true;
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error));
      return false;
    }
  }

  async function deleteQueue(queue: DownloadQueue) {
    try {
      await DeleteQueue(queue.id);
      if (selectedQueue() === queue.id) setSelectedQueue("all");
      setMessage(`${queue.name} deleted.`);
      await refresh();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : String(error));
    }
  }

  function clearSelection() {
    setSelection([]);
    setAnchor(null);
  }

  function pickRow(id: string, event: MouseEvent) {
    const visible = filtered().map((item) => item.id);
    if (event.shiftKey && anchor()) {
      const from = visible.indexOf(anchor()!);
      const to = visible.indexOf(id);
      if (from >= 0 && to >= 0) {
        const start = Math.min(from, to);
        const end = Math.max(from, to);
        setSelection(visible.slice(start, end + 1));
        return;
      }
    }
    setAnchor(id);
    if (event.metaKey || event.ctrlKey) {
      setSelection((current) =>
        current.includes(id)
          ? current.filter((entry) => entry !== id)
          : [...current, id],
      );
      return;
    }
    setSelection([id]);
  }

  function itemMenu(ids: string[]): MenuEntry[] {
    const chosen = items.filter((item) => ids.includes(item.id));
    if (!chosen.length) return [];
    const anyActive = chosen.some((item) => item.state === "active");
    const anyPaused = chosen.some((item) => item.state === "paused");
    const anyFailed = chosen.some((item) => item.state === "failed");
    const anyOpen = chosen.some((item) => item.state !== "complete");
    const anyComplete = chosen.some((item) => item.state === "complete");
    const single = chosen.length === 1 ? chosen[0] : null;
    const count = chosen.length;
    const plural = count === 1 ? "download" : `${count} downloads`;
    const selectedQueueID = single?.queueId;
    const isBulk = count > 1;

    const moveTargets: MenuEntry[] = queues.map((queue) => ({
      kind: "action",
      label: queue.name,
      hint:
        queue.id === selectedQueueID
          ? "current"
          : queue.running
            ? "running"
            : "stopped",
      disabled: queue.id === selectedQueueID,
      onSelect: () =>
        void bulk(
          (selected) => MoveToQueue(selected, queue.id),
          `Moved ${plural} to ${queue.name}.`,
        ),
    }));

    const entries: MenuEntry[] = [
      {
        kind: "action",
        label: "Pause",
        hint: anyActive ? undefined : "nothing is running",
        disabled: !anyActive,
        onSelect: () => void bulk((selected) => PauseBatch(selected), "Paused."),
      },
      {
        kind: "action",
        label: "Resume",
        hint: anyPaused ? undefined : "nothing is paused",
        disabled: !anyPaused,
        onSelect: () => void bulk((selected) => ResumeBatch(selected), "Resumed."),
      },
      {
        kind: "action",
        label: "Retry",
        hint: anyFailed ? undefined : "nothing has failed",
        disabled: !anyFailed,
        onSelect: () => void bulk((selected) => RetryBatch(selected), "Retrying."),
      },
      { kind: "separator" },
    ];

    if (single && single.state === "complete") {
      entries.push(
        {
          kind: "action",
          label: "Open file",
          onSelect: () => void action(() => OpenFile(single.id), "File opened."),
        },
        {
          kind: "action",
          label: "Open folder",
          onSelect: () => void action(() => OpenDirectory(single.id), "Folder opened."),
        },
        { kind: "separator" },
      );
    }

    entries.push(
      { kind: "group", label: "Move to queue", entries: moveTargets },
      { kind: "separator" },
      {
        kind: "action",
        label: isBulk ? "Cancel downloads" : "Cancel download",
        danger: true,
        disabled: !anyOpen,
        onSelect: () =>
          void bulk((selected) => CancelBatch(selected), "Downloads cancelled."),
      },
      {
        kind: "action",
        label: "Remove from list",
        danger: true,
        onSelect: () =>
          void bulk((selected) => RemoveBatch(selected), "Removed from the list."),
      },
    );

    if (anyComplete) {
      entries.push({
        kind: "action",
        label: isBulk ? "Delete files" : "Delete file",
        danger: true,
        onSelect: () =>
          void bulk((selected) => DeleteBatch(selected), "Files deleted."),
      });
    }

    return entries;
  }

  function openItemMenu(id: string, event: MouseEvent) {
    event.preventDefault();
    const ids = isSelected(id) ? selection() : [id];
    if (!isSelected(id)) {
      setSelection([id]);
      setAnchor(id);
    }
    setMenu({ x: event.clientX, y: event.clientY, entries: itemMenu(ids) });
  }

  function isSelected(id: string) {
    return selection().includes(id);
  }

  onMount(() => {
    void refresh();
    const offSnapshot = Events.On(EventQueueSnapshot, (event) =>
      applySnapshot(event.data as ServiceSnapshot),
    );
    const offProgress = Events.On(EventQueueProgress, (event) =>
      applyProgress(event.data as ItemProgress[]),
    );
    // Slow safety net: if an update is ever missed, the view reconciles itself.
    const timer = window.setInterval(() => void refresh(), 5000);
    onCleanup(() => {
      offSnapshot();
      offProgress();
      window.clearInterval(timer);
    });
  });

  return (
    <div class="shell">
      <div class="page">
        <header class="topbar rise">
          <p class="wordmark">
            ezdlr<span>.</span>
          </p>
          <p class="engine">
            <i /> aria2 · running locally
          </p>
        </header>

        <section class="intake rise rise-1">
          <p class="eyebrow">
            Intake — <b>direct links, http / https</b>
          </p>
          <h1 class="display">Paste a direct link.</h1>
          <p class="intake-sub">
            Single links land in the queue you pick and start as soon as it has
            room. Files land in your downloads folder.
          </p>
          <form class="intake-form" onSubmit={add}>
            <label class="field">
              <Icon name="link" size={16} />
              <input
                value={url()}
                onInput={(event) => setUrl(event.currentTarget.value)}
                type="url"
                placeholder="https://example.com/file.zip"
                aria-label="Direct link URL"
                required
              />
            </label>
            <div class="select-wrap intake-queue">
              <select
                value={intakeQueue()}
                onChange={(event) => setIntakeQueue(event.currentTarget.value)}
                aria-label="Add to queue"
              >
                <For each={queues}>
                  {(queue) => (
                    <option value={queue.id}>
                      {queue.name}
                      {queue.running ? "" : " · stopped"}
                    </option>
                  )}
                </For>
              </select>
              <span class="chevron">
                <Icon name="chevrondown" size={13} />
              </span>
            </div>
            <button class="btn-primary" disabled={busy()}>
              <Icon name="plus" size={15} /> Queue file
            </button>
          </form>
          <div class="intake-foot">
            <p class="status" role="status">
              {message()}
            </p>
            <button
              class="link-btn"
              onClick={() => void clipboard()}
              disabled={busy()}
            >
              Review clipboard <span class="arrow">→</span>
            </button>
          </div>

          <Show when={review()}>
            {(current) => (
              <section class="review">
                <div class="review-head">
                  <div>
                    <p class="eyebrow">Clipboard review</p>
                    <p class="review-counts">
                      {current().accepted.length} to add ·{" "}
                      {current().duplicates.length} duplicate ·{" "}
                      {current().rejected.length} rejected
                    </p>
                  </div>
                  <div class="review-actions">
                    <div class="select-wrap">
                      <select
                        value={reviewQueue()}
                        onChange={(event) => setReviewQueue(event.currentTarget.value)}
                        aria-label="Add clipboard links to queue"
                      >
                        <For each={queues}>
                          {(queue) => (
                            <option value={queue.id}>
                              {queue.name}
                              {queue.running ? "" : " · stopped"}
                            </option>
                          )}
                        </For>
                      </select>
                      <span class="chevron">
                        <Icon name="chevrondown" size={13} />
                      </span>
                    </div>
                    <button
                      onClick={() => void confirm()}
                      class="btn-primary small"
                      disabled={busy()}
                    >
                      Add {current().accepted.length} link
                      {current().accepted.length === 1 ? "" : "s"}
                    </button>
                    <button
                      onClick={() => {
                        setReview(null);
                        void CancelClipboardReview();
                      }}
                      class="btn-ghost"
                    >
                      Cancel
                    </button>
                  </div>
                </div>
                <p class="review-dest">
                  {(() => {
                    const queue = queues.find(
                      (entry) => entry.id === reviewQueue(),
                    );
                    if (!queue) return "Links are added without starting them.";
                    return queue.running
                      ? `${queue.name} is running, so these start as capacity allows.`
                      : `${queue.name} is stopped, so these wait until you start it.`;
                  })()}
                </p>
                <div class="review-list">
                  {[
                    ...current().accepted.map((entry) => ({
                      ...entry,
                      kind: "accepted",
                    })),
                    ...current().duplicates.map((entry) => ({
                      ...entry,
                      kind: "duplicate",
                    })),
                    ...current().rejected.map((entry) => ({
                      ...entry,
                      kind: "rejected",
                    })),
                  ].map((entry) => (
                    <div class="review-row" data-kind={entry.kind}>
                      <span class="dot" />
                      <span class="review-url" title={entry.url}>
                        {entry.url}
                      </span>
                      <span class="review-kind">
                        {entry.kind}
                        {entry.reason ? ` — ${entry.reason}` : ""}
                      </span>
                    </div>
                  ))}
                </div>
              </section>
            )}
          </Show>
        </section>

        <div class="workspace">
          <main class="rise rise-2">
            <QueueStrip
              queues={queues}
              counts={queueCounts()}
              selected={selectedQueue()}
              busy={busy()}
              onSelect={(id) => {
                setSelectedQueue(id);
                clearSelection();
              }}
              onCreate={createQueue}
              onRename={renameQueue}
              onDelete={(queue) => void deleteQueue(queue)}
              onAction={action}
              onOpenMenu={(next) => setMenu(next)}
            />

            <div class="queue-head">
              <div>
                <p class="eyebrow">
                  {selectedQueue() === "all" ? (
                    <>
                      Queue — <b>{padded(filtered().length)} files</b>
                    </>
                  ) : (
                    <>
                      {queueName(selectedQueue())} —{" "}
                      <b>{padded(filtered().length)} files</b>
                    </>
                  )}
                </p>
                <h2 class="display">In the pipeline</h2>
                <p class="queue-sub">
                  {selectedQueue() === "all"
                    ? "Moving, waiting, or ready to collect."
                    : `Filtered to ${queueName(selectedQueue())}.`}
                </p>
              </div>
              <div class="controls">
                <Show when={finishedCount() > 0}>
                  <button
                    class="clear-btn"
                    onClick={() =>
                      void action(
                        () => ClearCompleted(),
                        "Completed downloads cleared from the list.",
                      )
                    }
                    disabled={busy()}
                    title="Remove all completed downloads from the list"
                  >
                    <Icon name="trash" size={13} /> Clear completed
                  </button>
                </Show>
                <label class="search">
                  <Icon name="search" size={14} />
                  <input
                    value={query()}
                    onInput={(event) => setQuery(event.currentTarget.value)}
                    placeholder="Search downloads"
                    aria-label="Search downloads"
                  />
                </label>
                <div class="select-wrap">
                  <select
                    value={filter()}
                    onChange={(event) =>
                      setFilter(
                        event.currentTarget.value as "all" | DownloadState,
                      )
                    }
                    aria-label="Filter by state"
                  >
                    <option value="all">All states</option>
                    <option value="active">Downloading</option>
                    <option value="queued">Queued</option>
                    <option value="paused">Paused</option>
                    <option value="failed">Failed</option>
                    <option value="complete">Complete</option>
                  </select>
                  <span class="chevron">
                    <Icon name="chevrondown" size={13} />
                  </span>
                </div>
              </div>
            </div>

            <Show when={selection().length > 1}>
              <div class="bulk-bar">
                <span class="bulk-count">{selection().length} selected</span>
                <button
                  class="bulk-btn"
                  disabled={busy()}
                  onClick={() =>
                    void bulk((ids) => PauseBatch(ids), "Selection paused.")
                  }
                >
                  Pause
                </button>
                <button
                  class="bulk-btn"
                  disabled={busy()}
                  onClick={() =>
                    void bulk((ids) => ResumeBatch(ids), "Selection resumed.")
                  }
                >
                  Resume
                </button>
                <button
                  class="bulk-btn"
                  disabled={busy()}
                  onClick={() =>
                    void bulk((ids) => RetryBatch(ids), "Retrying selection.")
                  }
                >
                  Retry
                </button>
                <div class="select-wrap">
                  <select
                    value=""
                    onChange={(event) => {
                      const queueID = event.currentTarget.value;
                      event.currentTarget.value = "";
                      if (!queueID) return;
                      void bulk(
                        (ids) => MoveToQueue(ids, queueID),
                        `Moved to ${queueName(queueID)}.`,
                      );
                    }}
                    aria-label="Move selection to queue"
                  >
                    <option value="">Move to…</option>
                    <For each={queues}>
                      {(queue) => <option value={queue.id}>{queue.name}</option>}
                    </For>
                  </select>
                  <span class="chevron">
                    <Icon name="chevrondown" size={13} />
                  </span>
                </div>
                <button
                  class="bulk-btn danger"
                  disabled={busy()}
                  onClick={() =>
                    void bulk((ids) => CancelBatch(ids), "Downloads cancelled.")
                  }
                >
                  Cancel
                </button>
                <button
                  class="bulk-btn danger"
                  disabled={busy()}
                  onClick={() =>
                    void bulk((ids) => RemoveBatch(ids), "Removed from the list.")
                  }
                >
                  Remove
                </button>
                <button class="bulk-btn" onClick={clearSelection}>
                  Clear
                </button>
              </div>
            </Show>

            <div class="queue-list">
              <Show
                when={filtered().length > 0}
                fallback={
                  <div class="empty">
                    <span class="empty-icon">
                      <Icon name="link" size={19} />
                    </span>
                    <p class="empty-title">Nothing in the queue</p>
                    <p class="empty-sub">
                      Paste a link above and it lands here. Right-click a
                      download for its actions.
                    </p>
                  </div>
                }
              >
                <For each={filtered()}>
                  {(item, index) => (
                    <DownloadRow
                      item={item}
                      index={index()}
                      count={filtered().length}
                      connections={connections()}
                      busy={busy()}
                      selected={isSelected(item.id)}
                      queueName={queueName(item.queueId)}
                      onPick={pickRow}
                      onMenu={openItemMenu}
                      onAction={action}
                    />
                  )}
                </For>
              </Show>
            </div>
          </main>

          <aside class="rail rise rise-3">
            <section class="block">
              <p class="eyebrow">Engine</p>
              <div class="ledger">
                <div class="ledger-row">
                  <span class="ledger-label">Active</span>
                  <span
                    class={`ledger-value ${activeCount() > 0 ? "live" : "idle"}`}
                  >
                    {padded(activeCount())}
                  </span>
                </div>
                <div class="ledger-row">
                  <span class="ledger-label">Throughput</span>
                  <span
                    class={`ledger-value ${totalSpeed() > 0 ? "live" : "idle"}`}
                  >
                    {totalSpeed() > 0
                      ? `${formatBytes(totalSpeed())}/s`
                      : "—"}
                  </span>
                </div>
                <div class="ledger-row">
                  <span class="ledger-label">Finished</span>
                  <span class="ledger-value">{padded(finishedCount())}</span>
                </div>
                <div class="ledger-row">
                  <span class="ledger-label">Tracked</span>
                  <span class="ledger-value">{formatBytes(totalBytes())}</span>
                </div>
              </div>
              <p class="caption">
                {activeLimit()} at once · {connections()} connections per file
              </p>
            </section>

            <section class="block">
              <p class="eyebrow">
                <Icon name="folder" size={13} /> Destination
              </p>
              <p class="dest-path" title={directory()}>
                {directory()}
              </p>
              <p class="caption">All new transfers land here.</p>
            </section>

            <section class="block">
              <p class="eyebrow">Your control</p>
              <p class="control-copy">
                Queues only download while they are running. Clipboard links
                land in a stopped queue, so a batch never starts on its own.
              </p>
            </section>
          </aside>
        </div>

        <footer class="footer">
          <span>ezdlr — local transfer desk</span>
          <span>Private by design · http / https</span>
        </footer>
      </div>

      <Show when={menu()}>
        {(current) => <ContextMenu menu={current()} onClose={() => setMenu(null)} />}
      </Show>
    </div>
  );
}

render(() => <App />, document.getElementById("root")!);
