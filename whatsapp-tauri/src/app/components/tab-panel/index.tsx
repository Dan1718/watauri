import { useTab } from "@/app/hooks/use-tab";
import Chats from "./chats";
import Settings from "./settings";

export default function TabPanelSwitcher() {
  const { selectedTab } = useTab();

  if (selectedTab === "chats") {
    return <Chats selectedTab={selectedTab} />;
  }

  if (selectedTab === "settings") return <Settings />;

  return <div className="text-white">Coming soon...</div>;
}
