const form = document.querySelector("#add-form");
const input = document.querySelector("#url-input");
const message = document.querySelector("#message");
const list = document.querySelector("#queue-list");
const clipboardButton = document.querySelector("#clipboard-button");
const clipboardReview = document.querySelector("#clipboard-review");
const clipboardSummary = document.querySelector("#clipboard-summary");
const clipboardLinks = document.querySelector("#clipboard-links");
const clipboardConfirm = document.querySelector("#clipboard-confirm");
const clipboardCancel = document.querySelector("#clipboard-cancel");
let currentReview = null;
async function refresh() {
    const snapshot = await window.go.main.App.Snapshot();
    list.replaceChildren(...snapshot.items.map(renderItem));
}
function renderItem(item) {
    const element = document.createElement("article");
    element.className = "queue-item";
    const progress = item.totalBytes > 0 ? ` ${(item.completedBytes / item.totalBytes * 100).toFixed(1)}%` : "";
    const speed = item.downloadSpeed > 0 ? ` ${formatBytes(item.downloadSpeed)}/s` : "";
    element.innerHTML = `<div><strong>${escapeHTML(item.url)}</strong><span>${escapeHTML(item.destination)}</span></div><b>${item.state}${progress}${speed}</b>`;
    return element;
}
function formatBytes(value) {
    if (value < 1024)
        return `${value} B`;
    if (value < 1024 * 1024)
        return `${(value / 1024).toFixed(1)} KB`;
    return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}
function escapeHTML(value) {
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
    }
    catch (error) {
        message.textContent = error instanceof Error ? error.message : String(error);
    }
});
clipboardButton.addEventListener("click", async () => {
    message.textContent = "Reading clipboard...";
    try {
        const text = await navigator.clipboard.readText();
        currentReview = await window.go.main.App.ReviewClipboard(text);
        clipboardSummary.textContent = `${currentReview.accepted.length} accepted, ${currentReview.duplicates.length} duplicates, ${currentReview.rejected.length} rejected.`;
        const entries = [
            ...currentReview.accepted.map((entry) => ({ ...entry, status: "accepted" })),
            ...currentReview.duplicates.map((entry) => ({ ...entry, status: "duplicate" })),
            ...currentReview.rejected.map((entry) => ({ ...entry, status: "rejected" })),
        ];
        clipboardLinks.replaceChildren(...entries.map((entry) => {
            const item = document.createElement("p");
            item.textContent = `${entry.status}: ${entry.url}${entry.reason ? ` (${entry.reason})` : ""}`;
            return item;
        }));
        clipboardReview.hidden = false;
        message.textContent = "Review the links before adding them.";
    }
    catch (error) {
        message.textContent = error instanceof Error ? error.message : String(error);
    }
});
clipboardConfirm.addEventListener("click", async () => {
    if (currentReview === null)
        return;
    const result = await window.go.main.App.ConfirmClipboard(currentReview);
    const accepted = result.results.filter((entry) => entry.status === "accepted").length;
    const failures = result.results.filter((entry) => entry.status === "enqueue-failure").length;
    message.textContent = `${accepted} links added${failures ? `; ${failures} failed to enqueue` : "."}`;
    currentReview = null;
    clipboardReview.hidden = true;
    await refresh();
});
clipboardCancel.addEventListener("click", () => {
    currentReview = null;
    clipboardReview.hidden = true;
    void window.go.main.App.CancelClipboardReview();
    message.textContent = "Clipboard review cancelled.";
});
void refresh().catch((error) => {
    message.textContent = `Unable to load the queue: ${String(error)}`;
});
const refreshTimer = window.setInterval(() => {
    void refresh().catch((error) => {
        message.textContent = `Unable to refresh the queue: ${String(error)}`;
    });
}, 1000);
window.addEventListener("beforeunload", () => window.clearInterval(refreshTimer));
export {};
