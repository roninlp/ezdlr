import { For, Show, createSignal, onCleanup, onMount } from "solid-js";

export type MenuEntry =
  | {
      kind: "action";
      label: string;
      hint?: string;
      danger?: boolean;
      disabled?: boolean;
      onSelect: () => void;
    }
  | { kind: "separator" }
  | { kind: "group"; label: string; entries: MenuEntry[] };

export type MenuState = { x: number; y: number; entries: MenuEntry[] };

const EDGE = 8;

/**
 * Fixed-position context menu. It closes on the next click, Escape, scroll, or
 * resize, and flips near the viewport edges so it never renders off-screen.
 */
export function ContextMenu(props: { menu: MenuState; onClose: () => void }) {
  let frame: HTMLDivElement | undefined;
  const [position, setPosition] = createSignal({ x: props.menu.x, y: props.menu.y });
  const [flipSub, setFlipSub] = createSignal(false);

  function place() {
    if (!frame) return;
    const rect = frame.getBoundingClientRect();
    const x = Math.max(
      EDGE,
      Math.min(props.menu.x, window.innerWidth - rect.width - EDGE),
    );
    const y = Math.max(
      EDGE,
      Math.min(props.menu.y, window.innerHeight - rect.height - EDGE),
    );
    setPosition({ x, y });
    // A submenu opens to the right by default; flip it when that would run off
    // the edge of the window.
    setFlipSub(x + rect.width + 230 > window.innerWidth);
  }

  onMount(() => {
    place();
    const close = () => props.onClose();
    const onPointerDown = (event: MouseEvent) => {
      if (frame && !frame.contains(event.target as Node)) close();
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") close();
    };
    document.addEventListener("mousedown", onPointerDown, true);
    document.addEventListener("keydown", onKey);
    window.addEventListener("resize", close);
    window.addEventListener("wheel", close, { passive: true });
    onCleanup(() => {
      document.removeEventListener("mousedown", onPointerDown, true);
      document.removeEventListener("keydown", onKey);
      window.removeEventListener("resize", close);
      window.removeEventListener("wheel", close);
    });
  });

  return (
    <div
      ref={frame}
      class={`menu${flipSub() ? " menu-flip-sub" : ""}`}
      style={{ left: `${position().x}px`, top: `${position().y}px` }}
      role="menu"
      onContextMenu={(event) => event.preventDefault()}
    >
      <For each={props.menu.entries}>
        {(entry) =>
          entry.kind === "separator" ? (
            <div class="menu-sep" role="separator" />
          ) : entry.kind === "group" ? (
            <div class="menu-group" role="menuitem" tabindex={0}>
              <span class="menu-label">
                {entry.label}
                <span class="menu-arrow">
                  <svg
                    width="14"
                    height="14"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.8"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <path d="m9 18 6-6-6-6" />
                  </svg>
                </span>
              </span>
              <div class="menu menu-sub" role="menu">
                <For each={entry.entries}>
                  {(child) =>
                    child.kind === "separator" ? (
                      <div class="menu-sep" role="separator" />
                    ) : child.kind === "group" ? (
                      <div class="menu-item static" role="menuitem">
                        {child.label}
                      </div>
                    ) : (
                      <button
                        type="button"
                        role="menuitem"
                        class={`menu-item${child.danger ? " danger" : ""}`}
                        disabled={child.disabled}
                        onClick={() => {
                          props.onClose();
                          child.onSelect();
                        }}
                      >
                        <span>{child.label}</span>
                        <Show when={child.hint}>
                          <span class="menu-hint">{child.hint}</span>
                        </Show>
                      </button>
                    )
                  }
                </For>
              </div>
            </div>
          ) : (
            <button
              type="button"
              role="menuitem"
              class={`menu-item${entry.danger ? " danger" : ""}`}
              disabled={entry.disabled}
              onClick={() => {
                props.onClose();
                entry.onSelect();
              }}
            >
              <span>{entry.label}</span>
              <Show when={entry.hint}>
                <span class="menu-hint">{entry.hint}</span>
              </Show>
            </button>
          )
        }
      </For>
    </div>
  );
}
