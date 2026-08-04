import {
  For,
  Show,
  createMemo,
  createSignal,
  onCleanup,
  onMount,
} from "solid-js";
import { render } from "solid-js/web";
import { createStore, reconcile } from "solid-js/store";
import {
  AddURL,
  Cancel,
  CancelClipboardReview,
  ConfirmClipboard,
  MoveDown,
  MoveUp,
  Pause,
  ReadClipboard,
  Resume,
  Retry,
  ReviewClipboard,
  Snapshot,
} from "../wailsjs/go/main/App";
import { main } from "../wailsjs/go/models";
import "./index.css";

type DownloadState = "queued" | "active" | "paused" | "failed" | "complete";
type DownloadItem = main.DownloadItem;
type ClipboardReview = main.ClipboardReview;

const stateLabel: Record<DownloadState, string> = {
  active: "Downloading",
  queued: "Queued",
  paused: "Paused",
  failed: "Failed",
  complete: "Complete",
};

const stateGlyph: Record<DownloadState, string> = {
  active: "arrowdown",
  queued: "clock",
  paused: "pause",
  failed: "alert",
  complete: "check",
};

function formatBytes(value = 0) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 ** 2) return `${(value / 1024).toFixed(1)} KB`;
  if (value < 1024 ** 3) return `${(value / 1024 ** 2).toFixed(1)} MB`;
  return `${(value / 1024 ** 3).toFixed(2)} GB`;
}

function percent(item: DownloadItem) {
  return item.totalBytes > 0
    ? Math.min(100, (item.completedBytes / item.totalBytes) * 100)
    : 0;
}
function fileName(url: string) {
  try {
    return decodeURIComponent(
      new URL(url).pathname.split("/").filter(Boolean).pop() || url,
    );
  } catch {
    return url;
  }
}
function hostName(url: string) {
  try {
    return new URL(url).hostname;
  } catch {
    return url;
  }
}
function padded(value: number) {
  return value.toString().padStart(2, "0");
}

function Icon(props: { name: string; size?: number }) {
  const common = {
    width: props.size || 18,
    height: props.size || 18,
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    "stroke-width": 1.8,
    "stroke-linecap": "round",
    "stroke-linejoin": "round",
  } as const;
  const paths: Record<string, string> = {
    plus: "M12 5v14M5 12h14",
    link: "M10 13a5 5 0 0 0 7.07.07l2-2a5 5 0 0 0-7.07-7.07l-1.15 1.15M14 11a5 5 0 0 0-7.07-.07l-2 2A5 5 0 0 0 12 20l1.15-1.15",
    chevron: "m9 18 6-6-6-6",
    chevrondown: "m6 9 6 6 6-6",
    pause: "M8 5v14M16 5v14",
    play: "m9 5 7 7-7 7",
    x: "M6 6l12 12M18 6 6 18",
    retry: "M20 11a8.1 8.1 0 0 0-14.9-3L3 11m0 0V5m0 6h6",
    up: "m18 15-6-6-6 6",
    down: "m6 9 6 6 6-6",
    folder: "M3 7h7l2 2h9v10H3z",
    check: "m5 12 4 4L19 6",
    search: "M21 21l-4.35-4.35M11 19a8 8 0 1 1 0-16 8 8 0 0 1 0 16z",
    clock: "M12 6v6l4 2M12 22a10 10 0 1 1 0-20 10 10 0 0 1 0 20z",
    arrowdown: "M12 5v14M5 12l7 7 7-7",
    alert:
      "M12 9v4M12 17h.01M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z",
  };
  return (
    <svg {...common}>
      <path d={paths[props.name] || paths.link} />
    </svg>
  );
}

function App() {
  const [items, setItems] = createStore<DownloadItem[]>([]);
  const [directory, setDirectory] = createSignal("Downloads");
  const [connections, setConnections] = createSignal(4);
  const [activeLimit, setActiveLimit] = createSignal(3);
  const [url, setUrl] = createSignal("");
  const [query, setQuery] = createSignal("");
  const [filter, setFilter] = createSignal<"all" | DownloadState>("all");
  const [message, setMessage] = createSignal("Ready when you are.");
  const [busy, setBusy] = createSignal(false);
  const [review, setReview] = createSignal<ClipboardReview | null>(null);

  const filtered = createMemo(() => {
    const visible = items.filter(
      (item) =>
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

  async function refresh() {
    try {
      const snapshot = await Snapshot();
      setItems(reconcile(snapshot?.items || [], { key: "id" }));
      setDirectory(snapshot?.configuration?.downloadDirectory || "Downloads");
      if (snapshot?.configuration?.connections) {
        setConnections(snapshot.configuration.connections);
      }
      if (snapshot?.configuration?.activeLimit) {
        setActiveLimit(snapshot.configuration.activeLimit);
      }
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
      await AddURL(url());
      setUrl("");
      setMessage("Added. The queue will start it when capacity is available.");
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
      setReview(await ReviewClipboard(text));
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
    setBusy(true);
    try {
      const result = await ConfirmClipboard(current);
      const added = result.results.filter(
        (item) => item.status === "accepted",
      ).length;
      setReview(null);
      setMessage(`${added} link${added === 1 ? "" : "s"} added to the queue.`);
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

  onMount(() => {
    void refresh();
    const timer = window.setInterval(() => void refresh(), 1000);
    onCleanup(() => window.clearInterval(timer));
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
            It starts moving as soon as the queue has room. Files land in your
            downloads folder.
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
              onClick={clipboard}
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
                    <button
                      onClick={() => void confirm()}
                      class="btn-primary small"
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
            <div class="queue-head">
              <div>
                <p class="eyebrow">
                  Queue — <b>{padded(items.length)} files</b>
                </p>
                <h2 class="display">In the pipeline</h2>
                <p class="queue-sub">Moving, waiting, or ready to collect.</p>
              </div>
              <div class="controls">
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
                      Paste a link above and it lands here.
                    </p>
                  </div>
                }
              >
                <For each={filtered()}>
                  {(item, index) => (
                    <QueueItem
                      item={item}
                      index={index()}
                      count={filtered().length}
                      connections={connections()}
                      busy={busy()}
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
                Clipboard links are added only after you review them.
              </p>
            </section>
          </aside>
        </div>

        <footer class="footer">
          <span>ezdlr — local transfer desk</span>
          <span>Private by design · http / https</span>
        </footer>
      </div>
    </div>
  );
}

function SegmentStrip(props: {
  progress: number;
  cells: number;
  live: boolean;
}) {
  const filled = () =>
    Math.min(props.cells, Math.round((props.progress / 100) * props.cells));
  return (
    <div
      class="segs"
      role="img"
      aria-label={`${props.progress.toFixed(0)} percent downloaded`}
    >
      {Array.from({ length: props.cells }, (_, i) => (
        <span
          class={`seg${i < filled() ? " on" : ""}${
            props.live && i === filled() ? " next" : ""
          }`}
        />
      ))}
    </div>
  );
}

function QueueItem(props: {
  item: DownloadItem;
  index: number;
  count: number;
  connections: number;
  busy: boolean;
  onAction: (task: () => Promise<void>, success: string) => void;
}) {
  const item = () => props.item;
  const state = () =>
    (stateLabel[item().state as DownloadState]
      ? (item().state as DownloadState)
      : "queued") as DownloadState;
  const progress = () => percent(item());
  return (
    <article class="row" data-state={state()}>
      <span class="glyph">
        <Icon name={stateGlyph[state()]} size={15} />
      </span>
      <div class="row-main">
        <h3 class="row-title" title={item().url}>
          {fileName(item().url)}
        </h3>
        <p class="row-meta">
          {hostName(item().url)}
          <Show when={item().totalBytes > 0}>
            {" "}
            · {formatBytes(item().completedBytes)} of{" "}
            {formatBytes(item().totalBytes)}
          </Show>
          <Show when={item().downloadSpeed > 0}>
            {" "}
            · <span class="speed">{formatBytes(item().downloadSpeed)}/s</span>
          </Show>
          <Show when={item().attempts > 0}> · retry {item().attempts}</Show>
        </p>
        <Show
          when={
            item().state !== "complete" &&
            (item().state === "active" || item().totalBytes > 0)
          }
        >
          <SegmentStrip
            progress={progress()}
            cells={Math.max(1, props.connections) * 6}
            live={item().state === "active"}
          />
        </Show>
      </div>
      <div class="row-side">
        <span class="state-label">
          <i />
          {stateLabel[state()]}
        </span>
        <Show when={state() !== "complete" && state() !== "queued"}>
          <span class="row-pct">{progress().toFixed(0)}%</span>
        </Show>
        <div class="row-actions">
          <Show when={item().state === "queued"}>
            <button
              title="Move up"
              aria-label="Move up"
              disabled={props.index === 0 || props.busy}
              onClick={() =>
                props.onAction(
                  () => MoveUp(item().id),
                  "Queue priority updated.",
                )
              }
              class="icon-btn"
            >
              <Icon name="up" size={15} />
            </button>
            <button
              title="Move down"
              aria-label="Move down"
              disabled={props.index === props.count - 1 || props.busy}
              onClick={() =>
                props.onAction(
                  () => MoveDown(item().id),
                  "Queue priority updated.",
                )
              }
              class="icon-btn"
            >
              <Icon name="down" size={15} />
            </button>
          </Show>
          <Show when={item().state === "active"}>
            <button
              title="Pause"
              aria-label="Pause"
              disabled={props.busy}
              onClick={() =>
                props.onAction(() => Pause(item().id), "Download paused.")
              }
              class="icon-btn"
            >
              <Icon name="pause" size={15} />
            </button>
          </Show>
          <Show when={item().state === "paused"}>
            <button
              title="Resume"
              aria-label="Resume"
              disabled={props.busy}
              onClick={() =>
                props.onAction(() => Resume(item().id), "Download resumed.")
              }
              class="icon-btn"
            >
              <Icon name="play" size={15} />
            </button>
          </Show>
          <Show when={item().state === "failed"}>
            <button
              title="Retry"
              aria-label="Retry"
              disabled={props.busy}
              onClick={() =>
                props.onAction(() => Retry(item().id), "Retry started.")
              }
              class="icon-btn"
            >
              <Icon name="retry" size={15} />
            </button>
          </Show>
          <Show when={item().state !== "complete"}>
            <button
              title="Cancel"
              aria-label="Cancel"
              disabled={props.busy}
              onClick={() =>
                props.onAction(
                  () => Cancel(item().id),
                  "Download removed from queue.",
                )
              }
              class="icon-btn danger"
            >
              <Icon name="x" size={15} />
            </button>
          </Show>
        </div>
      </div>
    </article>
  );
}

render(() => <App />, document.getElementById("root")!);
