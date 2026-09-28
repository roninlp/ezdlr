import { For, Show, createSignal } from "solid-js";
import { StartQueue, StopQueue } from "../bindings/ezdlr/app";
import type { MenuState } from "./context-menu";
import type { DownloadQueue } from "./ui";
import { Icon, padded } from "./ui";

export type QueueCounts = Record<string, { total: number; active: number }>;

/**
 * The queue switcher: pick a queue to filter the list, start or stop it, and
 * manage it from its right-click menu. Custom queues are created stopped, so
 * nothing here starts downloading until a queue is started.
 */
export function QueueStrip(props: {
  queues: DownloadQueue[];
  counts: QueueCounts;
  selected: string;
  busy: boolean;
  onSelect: (id: string) => void;
  onCreate: (name: string) => Promise<boolean>;
  onRename: (queue: DownloadQueue, name: string) => Promise<boolean>;
  onDelete: (queue: DownloadQueue) => void;
  onAction: (task: () => Promise<void>, success: string) => void;
  onOpenMenu: (menu: MenuState) => void;
}) {
  const [creating, setCreating] = createSignal(false);
  const [draft, setDraft] = createSignal("");
  const [renaming, setRenaming] = createSignal<string | null>(null);
  const [renameDraft, setRenameDraft] = createSignal("");

  const all = () => {
    let total = 0;
    let active = 0;
    for (const key of Object.keys(props.counts)) {
      total += props.counts[key].total;
      active += props.counts[key].active;
    }
    return { total, active };
  };

  function beginRename(queue: DownloadQueue) {
    setRenaming(queue.id);
    setRenameDraft(queue.name);
  }

  async function submitRename(queue: DownloadQueue) {
    // Enter unmounts the field; the blur that follows must not submit twice.
    if (renaming() !== queue.id) return;
    const name = renameDraft().trim();
    setRenaming(null);
    if (!name || name === queue.name) return;
    await props.onRename(queue, name);
  }

  async function submitCreate() {
    if (!creating()) return;
    const name = draft().trim();
    if (!name) {
      setCreating(false);
      return;
    }
    if (await props.onCreate(name)) {
      setDraft("");
      setCreating(false);
    }
  }

  function queueMenu(queue: DownloadQueue, event: MouseEvent) {
    event.preventDefault();
    event.stopPropagation();
    const total = props.counts[queue.id]?.total ?? 0;
    props.onOpenMenu({
      x: event.clientX,
      y: event.clientY,
      entries: [
        {
          kind: "action",
          label: queue.running ? "Stop queue" : "Start queue",
          hint: queue.running ? "pauses its downloads" : "releases its downloads",
          onSelect: () =>
            props.onAction(
              () => (queue.running ? StopQueue(queue.id) : StartQueue(queue.id)),
              queue.running
                ? `${queue.name} stopped. Its downloads are held.`
                : `${queue.name} started.`,
            ),
        },
        { kind: "action", label: "Rename…", onSelect: () => beginRename(queue) },
        { kind: "separator" },
        {
          kind: "action",
          label: "Delete queue",
          danger: true,
          disabled: queue.builtIn || total > 0,
          hint: queue.builtIn ? "built in" : total > 0 ? "not empty" : undefined,
          onSelect: () => props.onDelete(queue),
        },
      ],
    });
  }

  return (
    <section class="queues-block">
      <div class="queues-head">
        <p class="eyebrow">
          Queues — <b>start or stop each one</b>
        </p>
        <Show when={!creating()}>
          <button
            type="button"
            class="queue-new"
            onClick={() => setCreating(true)}
            disabled={props.busy}
          >
            <Icon name="plus" size={13} /> New queue
          </button>
        </Show>
      </div>
      <div class="queues">
        <button
          type="button"
          class="queue-card"
          data-selected={props.selected === "all"}
          onClick={() => props.onSelect("all")}
        >
          <span class="queue-name">All downloads</span>
          <span class="queue-meta">
            {padded(all().total)} files
            <Show when={all().active > 0}>
              {" "}
              · <b>{padded(all().active)} live</b>
            </Show>
          </span>
        </button>

        <For each={props.queues}>
          {(queue) => {
            const counts = () => props.counts[queue.id] ?? { total: 0, active: 0 };
            return (
              <div
                class="queue-card"
                data-selected={props.selected === queue.id}
                data-stopped={!queue.running}
                role="button"
                tabindex={0}
                onClick={(event) => {
                  if ((event.target as HTMLElement).closest("button, input")) return;
                  props.onSelect(queue.id);
                }}
                onContextMenu={(event) => queueMenu(queue, event)}
                onKeyDown={(event) => {
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault();
                    props.onSelect(queue.id);
                  }
                }}
              >
                <Show
                  when={renaming() === queue.id}
                  fallback={
                    <>
                      <span class="queue-name">
                        <i class="queue-dot" data-running={queue.running} />
                        {queue.name}
                      </span>
                      <span class="queue-meta">
                        {padded(counts().total)} files
                        <Show when={counts().active > 0}>
                          {" "}
                          · <b>{padded(counts().active)} live</b>
                        </Show>
                      </span>
                    </>
                  }
                >
                  <input
                    class="queue-rename"
                    value={renameDraft()}
                    onInput={(event) => setRenameDraft(event.currentTarget.value)}
                    onClick={(event) => event.stopPropagation()}
                    onKeyDown={(event) => {
                      event.stopPropagation();
                      if (event.key === "Enter") void submitRename(queue);
                      if (event.key === "Escape") setRenaming(null);
                    }}
                    onBlur={() => void submitRename(queue)}
                    maxLength={48}
                    aria-label={`Rename ${queue.name}`}
                    ref={(element) => requestAnimationFrame(() => element.focus())}
                  />
                </Show>
                <button
                  type="button"
                  class="queue-toggle"
                  title={queue.running ? `Stop ${queue.name}` : `Start ${queue.name}`}
                  aria-label={queue.running ? `Stop ${queue.name}` : `Start ${queue.name}`}
                  disabled={props.busy}
                  onClick={(event) => {
                    event.stopPropagation();
                    props.onAction(
                      () => (queue.running ? StopQueue(queue.id) : StartQueue(queue.id)),
                      queue.running
                        ? `${queue.name} stopped. Its downloads are held.`
                        : `${queue.name} started.`,
                    );
                  }}
                >
                  <Icon name={queue.running ? "pause" : "play"} size={13} />
                </button>
              </div>
            );
          }}
        </For>

        <Show when={creating()}>
          <div class="queue-card creating">
            <input
              class="queue-rename"
              placeholder="Queue name"
              value={draft()}
              onInput={(event) => setDraft(event.currentTarget.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") void submitCreate();
                if (event.key === "Escape") {
                  setCreating(false);
                  setDraft("");
                }
              }}
              onBlur={() => void submitCreate()}
              maxLength={48}
              aria-label="New queue name"
              ref={(element) => requestAnimationFrame(() => element.focus())}
            />
            <span class="queue-meta">starts stopped</span>
          </div>
        </Show>
      </div>
      <Show when={props.selected !== "all"}>
        <p class="queues-hint">
          {(() => {
            const queue = props.queues.find((entry) => entry.id === props.selected);
            if (!queue) return "Filtered to one queue.";
            return queue.running
              ? `Filtered to ${queue.name} — it is running.`
              : `Filtered to ${queue.name} — it is stopped, so its downloads wait.`;
          })()}
        </p>
      </Show>
    </section>
  );
}
