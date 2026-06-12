import { useEffect, useRef, useState } from "react";
import { motion } from "motion/react";
import { ExternalLink, LayoutGrid, Lock, MoreVertical, Settings } from "lucide-react";
import { getAuthStatus, startAuth } from "../backend/client";

interface LoginScreenProps {
  onLogin: () => void;
}

export default function LoginScreen({ onLogin }: LoginScreenProps) {
  const [qrCode, setQrCode] = useState<string | null>(null);
  const [status, setStatus] = useState("connecting");
  const pollingRef = useRef(false);

  useEffect(() => {
    let active = true;

    async function init() {
      const initial = await getAuthStatus();
      if (!active) return;

      if (initial.status === "connected") {
        onLogin();
        return;
      }

      if (initial.qr) {
        setQrCode(initial.qr);
        setStatus(initial.status);
      }

      if (initial.status === "unauthenticated") {
        const started = await startAuth();
        if (!active) return;
        if (started.qr) setQrCode(started.qr);
        setStatus(started.status);
      }

      pollingRef.current = true;
    }

    init();

    const interval = setInterval(async () => {
      if (!active || !pollingRef.current) return;
      try {
        const result = await getAuthStatus();
        if (!active) return;
        if (result.qr) setQrCode(result.qr);
        setStatus(result.status);
        if (result.status === "connected") {
          pollingRef.current = false;
          onLogin();
        }
      } catch {
        // ignore polling errors
      }
    }, 2000);

    return () => {
      active = false;
      clearInterval(interval);
    };
  }, [onLogin]);

  return (
    <div className="flex min-h-screen flex-col bg-surface selection:bg-primary/30">
      <header className="z-50 flex w-full items-center justify-between px-12 py-8">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary-container">
            <LayoutGrid className="text-on-primary-container" size={24} fill="currentColor" />
          </div>
          <h1 className="text-xl font-extrabold tracking-tight text-on-surface">
            WhatsApp <span className="font-medium text-primary opacity-80">Tauri</span>
          </h1>
        </div>
        <div className="hidden gap-6 md:flex">
          <button className="text-sm font-medium text-on-surface-variant transition-colors hover:text-primary" type="button">
            Download
          </button>
          <button className="text-sm font-medium text-on-surface-variant transition-colors hover:text-primary" type="button">
            Privacy Policy
          </button>
        </div>
      </header>

      <main className="flex flex-grow items-center justify-center p-6 md:p-12">
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          className="relative grid w-full max-w-5xl grid-cols-1 items-center gap-12 overflow-hidden rounded-2xl bg-surface-container p-8 shadow-ambient md:grid-cols-2 md:p-16"
        >
          <div className="absolute -left-24 -top-24 h-64 w-64 rounded-full bg-primary opacity-5 blur-[100px]" />

          <div className="relative z-10 space-y-8">
            <div className="space-y-3">
              <h2 className="text-3xl font-bold tracking-tight text-on-surface">
                Use WhatsApp on your computer
              </h2>
              <p className="text-lg text-on-surface-variant">
                Pair your account with this device securely using a QR code.
              </p>
            </div>

            <ol className="space-y-6">
              {[
                {
                  number: 1,
                  text: (
                    <>
                      Open <span className="font-semibold text-on-surface">WhatsApp</span> on your phone
                    </>
                  ),
                },
                {
                  number: 2,
                  text: (
                    <>
                      Tap <span className="font-semibold text-on-surface">Menu</span>{" "}
                      <MoreVertical className="inline-block text-on-surface-variant" size={18} /> or{" "}
                      <span className="font-semibold text-on-surface">Settings</span>{" "}
                      <Settings className="inline-block text-on-surface-variant" size={18} /> and select{" "}
                      <span className="font-semibold text-on-surface">Linked Devices</span>
                    </>
                  ),
                },
                {
                  number: 3,
                  text: (
                    <>
                      Point your phone to this screen to <span className="font-semibold text-on-surface">capture the code</span>
                    </>
                  ),
                },
              ].map((step) => (
                <li key={step.number} className="flex items-start gap-4">
                  <span className="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full bg-surface-container-highest text-sm font-bold text-primary">
                    {step.number}
                  </span>
                  <p className="leading-relaxed text-on-surface-variant">{step.text}</p>
                </li>
              ))}
            </ol>

            <div className="pt-4">
              <button className="flex items-center gap-2 font-medium text-primary transition-colors hover:text-primary" type="button">
                Need help getting started?
                <ExternalLink size={14} />
              </button>
            </div>
          </div>

          <div className="relative z-10 flex flex-col items-center justify-center space-y-6">
            <div className="rounded-xl bg-primary-container p-2 shadow-ambient">
              <div className="relative rounded-lg bg-white p-4">
                {qrCode ? (
                  <img
                    src={qrCode}
                    alt="QR Code"
                    className="block h-64 w-64"
                  />
                ) : (
                  <div className="flex h-64 w-64 items-center justify-center">
                    <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
                  </div>
                )}
                <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
                  <div className="rounded-lg border border-gray-100 bg-white p-2 shadow-md">
                    <LayoutGrid size={32} className="text-primary-container" fill="currentColor" />
                  </div>
                </div>
              </div>
            </div>

            <div className="flex items-center gap-3 rounded-xl bg-surface-container-highest px-4 py-3">
              <div className="h-2 w-2 animate-pulse rounded-full bg-primary" />
              <span className="text-sm font-medium text-on-surface-variant">
                {status === "connected" ? "Connected!" : "Waiting for scan..."}
              </span>
            </div>

            <label className="group flex cursor-pointer items-center gap-2">
              <input
                type="checkbox"
                defaultChecked
                className="h-5 w-5 rounded border-outline-variant bg-surface-container-lowest text-primary transition-colors focus:ring-0"
                id="keep-signed-in"
              />
              <span className="text-sm text-on-surface-variant transition-colors group-hover:text-on-surface">
                Keep me signed in
              </span>
            </label>
          </div>
        </motion.div>
      </main>

      <footer className="flex w-full items-center justify-between border-t border-outline-variant/10 px-12 py-6 text-xs text-outline">
        <div className="flex items-center gap-4">
          <span>© 2024 WhatsApp Tauri Desktop</span>
          <span className="opacity-30">•</span>
          <span>Version 2.24.12.78</span>
        </div>
        <div className="flex items-center gap-2">
          <Lock size={14} />
          <span>Your personal messages are end-to-end encrypted</span>
        </div>
      </footer>
    </div>
  );
}
