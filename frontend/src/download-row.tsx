import { Show } from "solid-js";
import {
  Cancel,
  Delete,
  MoveDown,
  MoveUp,
  OpenDirectory,
  OpenFile,
  Pause,
  Remove,
  Resume,
  Retry,
} from "../bindings/ezdlr/app";
import type { DownloadItem } from "./ui";
import {
  Icon,
  asState,
  fileName,
  formatBytes,
  hostName,
  percent,
  stateGlyph,
  stateLabel,
} from "./ui";

function SegmentStrip(props: { progress: number; cells: number; live: boolean }) {
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

export function DownloadRow(props: {
  item: DownloadItem;
  index: number;
  count: number;
  connections: number;
  busy: boolean;
  selected: boolean;
  queueName: string;
  onPick: (id: string, event: MouseEvent) => void;
  onMenu: (id: string, event: MouseEvent) => void;
  onAction: (task: () => Promise<void>, success: string) => void;
}) {
  const item = () => props.item;
  const state = () => asState(item().state);
  const progress = () => percent(item());

  return (
    <article
      class="row"
      data-state={state()}
      data-selected={props.selected}
      onClick={(event) => {
        const target = event.target as HTMLElement;
        if (target.closest("button, select, input, a")) return;
        props.onPick(item().id, event);
      }}
      onContextMenu={(event) => props.onMenu(item().id, event)}
    >
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
          <Show when={item().queuePaused}>
            {" "}
            · held by a stopped queue
          </Show>
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
        <Show when={state() === "queued" || item().queuePaused}>
          <span class="row-queue" title={`Queue: ${props.queueName}`}>
            {props.queueName}
          </span>
        </Show>
        <div class="row-actions">
          <Show when={item().state === "queued"}>
            <button
              title="Move up"
              aria-label="Move up"
              disabled={props.index === 0 || props.busy}
              onClick={(event) => {
                event.stopPropagation();
                props.onAction(
                  () => MoveUp(item().id),
                  "Queue priority updated.",
                );
              }}
              class="icon-btn"
            >
              <Icon name="up" size={15} />
            </button>
            <button
              title="Move down"
              aria-label="Move down"
              disabled={props.index === props.count - 1 || props.busy}
              onClick={(event) => {
                event.stopPropagation();
                props.onAction(
                  () => MoveDown(item().id),
                  "Queue priority updated.",
                );
              }}
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
              onClick={(event) => {
                event.stopPropagation();
                props.onAction(() => Pause(item().id), "Download paused.");
              }}
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
              onClick={(event) => {
                event.stopPropagation();
                props.onAction(() => Resume(item().id), "Download resumed.");
              }}
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
              onClick={(event) => {
                event.stopPropagation();
                props.onAction(() => Retry(item().id), "Retry started.");
              }}
              class="icon-btn"
            >
              <Icon name="retry" size={15} />
            </button>
          </Show>
          <Show when={item().state === "complete"}>
            <button
              title="Open file"
              aria-label="Open file"
              disabled={props.busy}
              onClick={(event) => {
                event.stopPropagation();
                props.onAction(() => OpenFile(item().id), "File opened.");
              }}
              class="icon-btn"
            >
              <Icon name="external" size={15} />
            </button>
            <button
              title="Open folder"
              aria-label="Open folder"
              disabled={props.busy}
              onClick={(event) => {
                event.stopPropagation();
                props.onAction(
                  () => OpenDirectory(item().id),
                  "Folder opened.",
                );
              }}
              class="icon-btn"
            >
              <Icon name="folder" size={15} />
            </button>
            <button
              title="Remove from list"
              aria-label="Remove from list"
              disabled={props.busy}
              onClick={(event) => {
                event.stopPropagation();
                props.onAction(
                  () => Remove(item().id),
                  "Download removed from the list.",
                );
              }}
              class="icon-btn"
            >
              <Icon name="x" size={15} />
            </button>
            <button
              title="Delete file and remove from list"
              aria-label="Delete file and remove from list"
              disabled={props.busy}
              onClick={(event) => {
                event.stopPropagation();
                props.onAction(
                  () => Delete(item().id),
                  "Download and its file were deleted.",
                );
              }}
              class="icon-btn danger"
            >
              <Icon name="trash" size={15} />
            </button>
          </Show>
          <Show when={item().state !== "complete"}>
            <button
              title="Cancel"
              aria-label="Cancel"
              disabled={props.busy}
              onClick={(event) => {
                event.stopPropagation();
                props.onAction(
                  () => Cancel(item().id),
                  "Download removed from queue.",
                );
              }}
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
