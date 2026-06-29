"use client";

import { useEffect, useRef, useState } from "react";
import TabActivePanel from "./components/tab-active-panel";
import TabIcons from "./components/tab-icons";
import TabPanel from "./components/tab-panel";
import ChatsProvider from "./context/chats-provider";
import ContactsProvider from "./context/contacts-provider";
import CurrentChatProvider from "./context/current-chat-provider";
import NewChatProvider from "./context/new-chat-provider";
import ProfileProvider from "./context/profile-provider";
import TabProvider from "./context/tab-provider";

const API_BASE = "http://localhost:8090";

type AuthStatusResponse = {
  status: string;
  qr: string;
};

async function getAuthStatus(): Promise<AuthStatusResponse> {
  const response = await fetch(`${API_BASE}/api/auth/status`);
  if (!response.ok) throw new Error(`auth status failed: ${response.status}`);
  return response.json();
}

async function startAuth(): Promise<AuthStatusResponse> {
  const response = await fetch(`${API_BASE}/api/auth/start`, { method: "POST" });
  if (!response.ok) throw new Error(`auth start failed: ${response.status}`);
  return response.json();
}

export default function Home() {
  const [connected, setConnected] = useState(false);
  const [qrCode, setQrCode] = useState<string | null>(null);
  const [status, setStatus] = useState("connecting");
  const [chatListWidth, setChatListWidth] = useState(() =>
    typeof window === "undefined" ? 420 : Math.min(560, Math.max(320, window.innerWidth - 500))
  );
  const chatListRef = useRef<HTMLDivElement>(null);
  const resizingRef = useRef(false);
  const pollingRef = useRef(false);

  useEffect(() => {
    const stopResize = () => {
      resizingRef.current = false;
    };

    const resize = (event: PointerEvent) => {
      if (!resizingRef.current || !chatListRef.current) return;
      const left = chatListRef.current.getBoundingClientRect().left;
      const max = Math.max(320, window.innerWidth - left - 420);
      setChatListWidth(Math.min(Math.max(event.clientX - left, 320), max));
    };

    window.addEventListener("pointermove", resize);
    window.addEventListener("pointerup", stopResize);

    return () => {
      window.removeEventListener("pointermove", resize);
      window.removeEventListener("pointerup", stopResize);
    };
  }, []);

  useEffect(() => {
    let active = true;
    let retryTimer: ReturnType<typeof setTimeout> | null = null;

    async function init() {
      try {
        const initial = await getAuthStatus();
        if (!active) return;

        if (initial.status === "connected") {
          setConnected(true);
          return;
        }

        if (initial.qr) setQrCode(initial.qr);

        if (initial.status === "unauthenticated") {
          const started = await startAuth();
          if (!active) return;
          if (started.qr) setQrCode(started.qr);
          setStatus(started.status);
        } else {
          setStatus(initial.status);
        }

        pollingRef.current = true;
      } catch {
        if (!active) return;
        setStatus("waiting for backend");
        retryTimer = setTimeout(() => void init(), 1000);
      }
    }

    void init();

    const interval = setInterval(async () => {
      if (!active || !pollingRef.current) return;
      try {
        const result = await getAuthStatus();
        if (!active) return;
        if (result.qr) setQrCode(result.qr);
        setStatus(result.status);
        if (result.status === "connected") {
          pollingRef.current = false;
          setConnected(true);
        }
      } catch {
        setStatus("waiting for backend");
      }
    }, 2000);

    return () => {
      active = false;
      if (retryTimer) clearTimeout(retryTimer);
      clearInterval(interval);
    };
  }, []);

  if (!connected) {
    return <LoginScreen qrCode={qrCode} status={status} />;
  }

  return (
    <ProfileProvider>
      <TabProvider>
        <ContactsProvider>
          <ChatsProvider>
            <CurrentChatProvider>
              <NewChatProvider>
                <section className="flex h-full w-full overflow-hidden">
                  <div className="h-full w-20 shrink-0">
                    <TabIcons />
                  </div>
                  <div
                    className="relative h-full shrink-0"
                    ref={chatListRef}
                    style={{ width: chatListWidth }}
                  >
                    <TabPanel />
                    <div
                      aria-orientation="vertical"
                      className="absolute -right-1 top-0 z-50 h-full w-2 cursor-col-resize bg-transparent transition-colors hover:bg-emerald-400/20 active:bg-emerald-400/30"
                      onPointerDown={(event) => {
                        event.preventDefault();
                        resizingRef.current = true;
                      }}
                      role="separator"
                    />
                  </div>
                  <div className="min-w-0 flex-1">
                    <TabActivePanel />
                  </div>
                </section>
              </NewChatProvider>
            </CurrentChatProvider>
          </ChatsProvider>
        </ContactsProvider>
      </TabProvider>
    </ProfileProvider>
  );
}

function LoginScreen({ qrCode, status }: { qrCode: string | null; status: string }) {
  return (
    <main className="flex h-full w-full items-center justify-center bg-[#141414] text-white">
      <section className="grid w-full max-w-5xl grid-cols-1 items-center gap-12 rounded-3xl border border-white/10 bg-[#1b1b1b] p-10 md:grid-cols-2 md:p-16">
        <div className="space-y-8">
          <div>
            <h1 className="text-3xl font-semibold">Use WhatsApp on your computer</h1>
            <p className="mt-4 text-lg text-white/60">Pair your account with this device securely using a QR code.</p>
          </div>
          <ol className="space-y-5 text-white/65">
            <li>1. Open WhatsApp on your phone</li>
            <li>2. Tap Menu or Settings and select Linked Devices</li>
            <li>3. Point your phone to this screen to capture the code</li>
          </ol>
        </div>
        <div className="flex flex-col items-center gap-6">
          <div className="rounded-2xl bg-white p-4">
            {qrCode ? (
              // ponytail: backend returns a QR data URL; next/image does not help here.
              // eslint-disable-next-line @next/next/no-img-element
              <img alt="QR Code" className="h-64 w-64" src={qrCode} />
            ) : (
              <div className="flex h-64 w-64 items-center justify-center text-sm text-black/50">Loading QR...</div>
            )}
          </div>
          <p className="rounded-full bg-white/10 px-4 py-2 text-sm capitalize text-white/70">{status}</p>
        </div>
      </section>
    </main>
  );
}
