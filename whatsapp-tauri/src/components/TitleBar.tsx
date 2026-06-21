import type { ReactNode } from "react";
import { useEffect, useMemo, useState } from "react";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { backendHealth } from "../backend/client";
import { Maximize2, Minimize2, X } from "lucide-react";

export default function TitleBar() {
  const appWindow = useMemo(() => {
    try {
      return getCurrentWindow();
    } catch {
      return null;
    }
  }, []);
  const [maximized, setMaximized] = useState(false);
  const [mode, setMode] = useState("mock");

  console.log(`[titlebar] 🔄 Render: mode=${mode} maximized=${maximized} tauri=${!!appWindow}`);

  useEffect(() => {
    let active = true;
    const start = performance.now();
    console.log("[titlebar] ⚡ Health check starting...");

    void backendHealth().then((health) => {
      const elapsed = (performance.now() - start).toFixed(1);
      if (active) {
        console.log(`[titlebar] ✅ Health check: mode=${health.mode} status=${health.status} (${elapsed}ms)`);
        setMode(health.mode);
      } else {
        console.log(`[titlebar] ⏭️ Health check completed but unmounted (${elapsed}ms)`);
      }
    });

    return () => {
      active = false;
    };
  }, []);

  const handleMinimize = async () => {
    console.log("[titlebar] ➖ Minimize");
    await appWindow?.minimize();
  };

  const handleToggleMaximize = async () => {
    if (!appWindow) return;
    const wasMaximized = await appWindow.isMaximized();
    console.log(`[titlebar] 🔲 Toggle maximize: ${wasMaximized} -> ${!wasMaximized}`);
    await appWindow.toggleMaximize();
    setMaximized(await appWindow.isMaximized());
  };

  const handleClose = async () => {
    console.log("[titlebar] ❌ Close");
    await appWindow?.close();
  };

  return (
    <header
      data-tauri-drag-region
      className="flex h-10 shrink-0 items-center justify-between border-b border-outline-variant/10 bg-surface-container-high pl-3"
    >
      <div data-tauri-drag-region className="flex items-center gap-2 text-xs font-medium tracking-wide text-on-surface-variant">
        <span>WhatsApp Tauri</span>
        <span className="rounded-full bg-surface-container-highest px-2 py-0.5 text-[10px] uppercase tracking-widest text-primary">
          {mode}
        </span>
      </div>

      <div className="flex items-stretch">
        <WindowButton label="Minimize" onClick={handleMinimize}>
          <Minimize2 size={14} />
        </WindowButton>
        <WindowButton label={maximized ? "Restore" : "Maximize"} onClick={handleToggleMaximize}>
          <Maximize2 size={14} />
        </WindowButton>
        <WindowButton label="Close" onClick={handleClose} close>
          <X size={14} />
        </WindowButton>
      </div>
    </header>
  );
}

interface WindowButtonProps {
  label: string;
  close?: boolean;
  onClick: () => void | Promise<void>;
  children: ReactNode;
}

function WindowButton({ label, close = false, onClick, children }: WindowButtonProps) {
  return (
    <button
      aria-label={label}
      type="button"
      onClick={() => void onClick()}
      className={`flex h-10 w-12 cursor-pointer items-center justify-center text-on-surface-variant transition-colors ${close ? "hover:bg-[#c42b1c] hover:text-white" : "hover:bg-surface-container-highest hover:text-on-surface"}`}
    >
      {children}
    </button>
  );
}
