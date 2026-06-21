import { useState } from "react";
import { LogOut, AlertTriangle, X } from "lucide-react";
import { logout } from "../backend/client";

interface SettingsScreenProps {
  onClose: () => void;
  onLogout: () => void;
}

export default function SettingsScreen({ onClose, onLogout }: SettingsScreenProps) {
  const [confirming, setConfirming] = useState(false);
  const [loggingOut, setLoggingOut] = useState(false);

  const handleLogout = async () => {
    setLoggingOut(true);
    try {
      await logout();
      onLogout();
    } catch (err) {
      console.error("[settings] Logout failed:", err);
      setLoggingOut(false);
    }
  };

  return (
    <div className="relative flex flex-1 flex-col bg-surface">
      <header className="flex h-[60px] items-center justify-between bg-surface-container-high px-4">
        <h1 className="text-xl font-bold tracking-tight text-on-surface">Settings</h1>
        <button
          onClick={onClose}
          className="rounded-full p-2 text-outline transition-colors hover:bg-surface-container-highest"
          type="button"
        >
          <X size={20} />
        </button>
      </header>

      <div className="flex flex-1 items-start justify-center p-8">
        <div className="w-full max-w-md space-y-6">
          <div className="rounded-xl bg-surface-container p-6">
            <h2 className="mb-2 text-lg font-semibold text-on-surface">Account</h2>
            <p className="mb-6 text-sm text-on-surface-variant">
              Log out of WhatsApp Desktop on this device. Your chats will remain on your phone.
            </p>

            {!confirming ? (
              <button
                onClick={() => setConfirming(true)}
                className="flex w-full items-center justify-center gap-2 rounded-lg bg-error px-4 py-3 font-medium text-on-surface transition-colors hover:opacity-90"
                type="button"
              >
                <LogOut size={18} />
                Log out
              </button>
            ) : (
              <div className="space-y-4 rounded-lg border border-error/30 bg-error/10 p-4">
                <div className="flex items-start gap-3">
                  <AlertTriangle size={20} className="mt-0.5 shrink-0 text-error" />
                  <div>
                    <h3 className="font-medium text-on-surface">Confirm logout</h3>
                    <p className="mt-1 text-sm text-on-surface-variant">
                      This will revoke the session on this device. You'll need to scan the QR code again to reconnect.
                    </p>
                  </div>
                </div>
                <div className="flex gap-3">
                  <button
                    onClick={() => setConfirming(false)}
                    className="flex-1 rounded-lg bg-surface-container-highest px-4 py-2 text-sm font-medium text-on-surface transition-colors hover:opacity-80"
                    type="button"
                    disabled={loggingOut}
                  >
                    Cancel
                  </button>
                  <button
                    onClick={handleLogout}
                    className="flex-1 rounded-lg bg-error px-4 py-2 text-sm font-medium text-on-surface transition-colors hover:opacity-90"
                    type="button"
                    disabled={loggingOut}
                  >
                    {loggingOut ? "Logging out..." : "Yes, log out"}
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
