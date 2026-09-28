import type {
  DownloadItem as GeneratedDownloadItem,
  DownloadQueue as GeneratedDownloadQueue,
  ServiceSnapshot as GeneratedServiceSnapshot,
} from "../bindings/ezdlr/models";

export type DownloadState = "queued" | "active" | "paused" | "failed" | "complete";

// The Go enum carries a zero value that the UI does not model; runtime values
// always match this local union.
export type DownloadItem = Omit<GeneratedDownloadItem, "state"> & {
  state: DownloadState;
};

export type DownloadQueue = GeneratedDownloadQueue;
export type ServiceSnapshot = GeneratedServiceSnapshot;

/**
 * Counter-only update pushed on the progress event. It mirrors the Go
 * `ItemProgress` model, which never crosses a bound method and so is not
 * emitted by the binding generator.
 */
export type ItemProgress = {
  id: string;
  state: string;
  totalBytes: number;
  completedBytes: number;
  downloadSpeed: number;
  attempts: number;
  path?: string;
};

/** Event names the Go engine loop pushes queue updates on. */
export const EventQueueSnapshot = "ezdlr:queue:snapshot";
export const EventQueueProgress = "ezdlr:queue:progress";

export const MainQueueID = "main";
export const ClipboardQueueID = "clipboard";

export const stateLabel: Record<DownloadState, string> = {
  active: "Downloading",
  queued: "Queued",
  paused: "Paused",
  failed: "Failed",
  complete: "Complete",
};

export const stateGlyph: Record<DownloadState, string> = {
  active: "arrowdown",
  queued: "clock",
  paused: "pause",
  failed: "alert",
  complete: "check",
};

/** Unknown states from the Go side fall back to the queued presentation. */
export function asState(value: unknown): DownloadState {
  return typeof value === "string" && value in stateLabel
    ? (value as DownloadState)
    : "queued";
}

export function formatBytes(value = 0) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 ** 2) return `${(value / 1024).toFixed(1)} KB`;
  if (value < 1024 ** 3) return `${(value / 1024 ** 2).toFixed(1)} MB`;
  return `${(value / 1024 ** 3).toFixed(2)} GB`;
}

export function percent(item: DownloadItem) {
  return item.totalBytes > 0
    ? Math.min(100, (item.completedBytes / item.totalBytes) * 100)
    : 0;
}

export function fileName(url: string) {
  try {
    return decodeURIComponent(
      new URL(url).pathname.split("/").filter(Boolean).pop() || url,
    );
  } catch {
    return url;
  }
}

export function hostName(url: string) {
  try {
    return new URL(url).hostname;
  } catch {
    return url;
  }
}

export function padded(value: number) {
  return value.toString().padStart(2, "0");
}

export function snapshotItems(snapshot: ServiceSnapshot | null | undefined) {
  return ((snapshot?.items ?? []) as unknown as DownloadItem[]);
}

export function snapshotQueues(snapshot: ServiceSnapshot | null | undefined) {
  return ((snapshot?.queues ?? []) as unknown as DownloadQueue[]);
}

export function Icon(props: { name: string; size?: number }) {
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
    external:
      "M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6M15 3h6v6M10 14 21 3",
    trash:
      "M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M10 11v6M14 11v6",
    layers: "M12 3 3 8l9 5 9-5-9-5ZM3 13l9 5 9-5M3 18l9 5 9-5",
  };
  return (
    <svg {...common}>
      <path d={paths[props.name] || paths.link} />
    </svg>
  );
}
