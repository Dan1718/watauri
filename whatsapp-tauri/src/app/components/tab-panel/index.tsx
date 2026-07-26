import { useTab } from "@/app/hooks/use-tab";
import Chats from "./chats";
import Settings from "./settings";

export default function TabPanelSwitcher() {
  const { selectedTab } = useTab();
  const showChats = selectedTab === "chats";

  return (
    <div className="relative h-full w-full">
      <div className={`absolute inset-0 ${showChats ? "visible" : "invisible pointer-events-none"}`}>
        <Chats selectedTab="chats" />
      </div>
      {!showChats && (selectedTab === "settings"
        ? <Settings />
        : <div className="text-white">Coming soon...</div>)}
    </div>
  );
}
