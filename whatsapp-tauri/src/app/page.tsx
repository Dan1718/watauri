"use client";

import dynamic from "next/dynamic";
import { useEffect, useRef, useState } from "react";

const AuthenticatedShell = dynamic(() => import("./components/authenticated-shell"), {
  ssr: false,
  loading: () => <main className="h-full w-full bg-[#141414]" />,
});

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
  const pollingRef = useRef(false);

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

  return <AuthenticatedShell />;
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
