import { useEffect, useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { listChats } from "./backend/client";
import LoginScreen from "./components/LoginScreen";
import MessageArea from "./components/MessageArea";
import NavigationRail from "./components/NavigationRail";
import Sidebar from "./components/Sidebar";
import TitleBar from "./components/TitleBar";
import type { AppScreen, Chat } from "./types";

export default function App() {
  const [screen, setScreen] = useState<AppScreen>("login");
  const [activeChatId, setActiveChatId] = useState<string | null>(null);
  const [chats, setChats] = useState<Chat[]>([]);

  useEffect(() => {
    let active = true;

    void listChats().then((nextChats) => {
      if (active) {
        setChats(nextChats);
      }
    });

    return () => {
      active = false;
    };
  }, []);

  const activeChat = chats.find((chat) => chat.id === activeChatId) || null;

  const handleLogin = () => {
    setScreen("chats");
  };

  const handleSelectChat = (id: string) => {
    setActiveChatId(id);
  };

  const handleUnarchive = (id: string) => {
    setChats((previousChats) =>
      previousChats.map((chat) =>
        chat.id === id ? { ...chat, isArchived: false } : chat,
      ),
    );
  };

  if (screen === "login") {
    return <LoginScreen onLogin={handleLogin} />;
  }

  return (
    <div className="flex h-screen w-full flex-col overflow-hidden bg-surface">
      <TitleBar />

      <div className="flex min-h-0 flex-1 overflow-hidden">
        <NavigationRail
          currentScreen={screen}
          setScreen={(nextScreen) => {
            setScreen(nextScreen);
            if (screen !== nextScreen) {
              setActiveChatId(null);
            }
          }}
        />

        <main className="flex min-h-0 flex-1 overflow-hidden">
          <Sidebar
            chats={chats}
            screen={screen}
            activeChatId={activeChatId}
            onSelectChat={handleSelectChat}
            onBack={() => setScreen("chats")}
          />

          <MessageArea screen={screen} chat={activeChat} onUnarchive={handleUnarchive} />
        </main>

        <AnimatePresence>
          {screen === "chats" && !activeChatId ? (
            <motion.div
              initial={{ opacity: 0, y: 50, scale: 0.9 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, scale: 0.9 }}
              className="fixed bottom-8 right-8 z-[100] hidden max-w-sm items-center gap-4 rounded-lg border border-outline-variant/30 bg-surface-bright p-4 shadow-ambient md:flex"
            >
              <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary-container text-on-primary-container">
                <LaptopIcon />
              </div>
              <div>
                <h4 className="text-sm font-bold text-on-surface">
                  WhatsApp Desktop is ready
                </h4>
                <p className="text-xs text-on-surface-variant">
                  Get notified of new messages even when your phone is offline.
                </p>
              </div>
              <button className="p-1 text-outline hover:text-on-surface" type="button">
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="18"
                  height="18"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <path d="M18 6 6 18" />
                  <path d="m6 6 12 12" />
                </svg>
              </button>
            </motion.div>
          ) : null}
        </AnimatePresence>
      </div>
    </div>
  );
}

function LaptopIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <rect width="18" height="12" x="3" y="4" rx="2" ry="2" />
      <line x1="2" x2="22" y1="20" y2="20" />
    </svg>
  );
}
