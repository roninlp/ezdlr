type DownloadState = "queued" | "active" | "paused" | "failed" | "complete";

type DownloadItem = {
  id: string;
  url: string;
  state: DownloadState;
  destination: string;
  addedAt: string;
  gid?: string;
  totalBytes: number;
  completedBytes: number;
  downloadSpeed: number;
};

type ServiceSnapshot = {
  items: DownloadItem[];
  configuration: { downloadDirectory: string };
};

export {};

declare global {
  interface Window {
    go: {
      main: {
        App: {
          AddURL(url: string): Promise<DownloadItem>;
          Snapshot(): Promise<ServiceSnapshot>;
        };
      };
    };
  }
}

const form = document.querySelector<HTMLFormElement>("#add-form")!;
const input = document.querySelector<HTMLInputElement>("#url-input")!;
const message = document.querySelector<HTMLParagraphElement>("#message")!;
const list = document.querySelector<HTMLDivElement>("#queue-list")!;

async function refresh(): Promise<void> {
  const snapshot = await window.go.main.App.Snapshot();
  list.replaceChildren(...snapshot.items.map(renderItem));
}

function renderItem(item: DownloadItem): HTMLElement {
  const element = document.createElement("article");
  element.className = "queue-item";
  const progress = item.totalBytes > 0 ? ` ${(item.completedBytes / item.totalBytes * 100).toFixed(1)}%` : "";
  const speed = item.downloadSpeed > 0 ? ` ${formatBytes(item.downloadSpeed)}/s` : "";
  element.innerHTML = `<div><strong>${escapeHTML(item.url)}</strong><span>${escapeHTML(item.destination)}</span></div><b>${item.state}${progress}${speed}</b>`;
  return element;
}

function formatBytes(value: number): string {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

function escapeHTML(value: string): string {
  return value.replace(/[&<>'"]/g, (character) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;",
  })[character] ?? character);
}

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  message.textContent = "Adding link...";
  try {
    await window.go.main.App.AddURL(input.value);
    input.value = "";
    message.textContent = "Added to the queue.";
    await refresh();
  } catch (error) {
    message.textContent = error instanceof Error ? error.message : String(error);
  }
});

void refresh().catch((error: unknown) => {
  message.textContent = `Unable to load the queue: ${String(error)}`;
});
