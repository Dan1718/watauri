import { useRef } from "react";
import { Archive, LayoutGrid, MessageSquare, Settings, Star, Users2 } from "lucide-react";
import { CURRENT_USER } from "../constants";
import type { AppScreen } from "../types";

interface NavigationRailProps {
  currentScreen: AppScreen;
  setScreen: (screen: AppScreen) => void;
}

export default function NavigationRail({ currentScreen, setScreen }: NavigationRailProps) {
  const lastScreen = useRef(currentScreen);

  if (lastScreen.current !== currentScreen) {
    console.log(`[nav] 🔀 Screen: ${lastScreen.current} -> ${currentScreen}`);
    lastScreen.current = currentScreen;
  }

  const navigate = (screen: AppScreen) => {
    console.log(`[nav] 🔘 Navigate: ${currentScreen} -> ${screen}`);
    setScreen(screen);
  };

  return (
    <nav className="flex h-full w-[72px] shrink-0 flex-col items-center justify-between bg-surface-container py-4">
      <div className="flex w-full flex-col items-center gap-4 px-2">
        <div className="mb-2 text-xl font-bold text-primary">
          <LayoutGrid className="text-3xl" size={32} fill="currentColor" />
        </div>

        <button
          onClick={() => navigate("chats")}
          className={`group relative rounded-lg p-3 transition-colors active:scale-90 ${currentScreen === "chats" ? "bg-surface-container-highest text-primary" : "text-outline hover:bg-surface-container-high"}`}
          title="Chats"
          type="button"
        >
          <MessageSquare size={24} fill={currentScreen === "chats" ? "currentColor" : "none"} />
        </button>

        <button
          onClick={() => navigate("communities")}
          className={`rounded-lg p-3 transition-colors active:scale-90 ${currentScreen === "communities" ? "bg-surface-container-highest text-primary" : "text-outline hover:bg-surface-container-high"}`}
          title="Communities"
          type="button"
        >
          <Users2 size={24} />
        </button>
      </div>

      <div className="flex w-full flex-col items-center gap-4 px-2">
        <button
          onClick={() => navigate("starred")}
          className={`rounded-lg p-3 transition-colors active:scale-90 ${currentScreen === "starred" ? "bg-surface-container-highest text-primary" : "text-outline hover:bg-surface-container-high"}`}
          title="Starred"
          type="button"
        >
          <Star size={24} fill={currentScreen === "starred" ? "currentColor" : "none"} />
        </button>

        <button
          onClick={() => navigate("archived")}
          className={`rounded-lg p-3 transition-colors active:scale-90 ${currentScreen === "archived" ? "bg-surface-container-highest text-primary" : "text-outline hover:bg-surface-container-high"}`}
          title="Archive"
          type="button"
        >
          <Archive size={24} fill={currentScreen === "archived" ? "currentColor" : "none"} />
        </button>

        <button
          onClick={() => navigate("settings")}
          className={`mb-2 rounded-lg p-3 transition-colors active:scale-90 ${currentScreen === "settings" ? "bg-surface-container-highest text-primary" : "text-outline hover:bg-surface-container-high"}`}
          title="Settings"
          type="button"
        >
          <Settings size={24} />
        </button>

        <div className="h-9 w-9 cursor-pointer overflow-hidden rounded-full border border-outline-variant transition-colors hover:border-primary">
          <img className="h-full w-full object-cover" src={CURRENT_USER.avatar} alt="My Profile" referrerPolicy="no-referrer" />
        </div>
      </div>
    </nav>
  );
}
