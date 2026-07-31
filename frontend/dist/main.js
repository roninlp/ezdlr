const form = document.querySelector("#add-form");
const input = document.querySelector("#url-input");
const message = document.querySelector("#message");
const list = document.querySelector("#queue-list");
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
void refresh().catch((error) => {
    message.textContent = `Unable to load the queue: ${String(error)}`;
});
export {};
