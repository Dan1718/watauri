import { useTab } from "@/app/hooks/use-tab";
import { GearSixIcon } from "@phosphor-icons/react";
import { memo } from "react";
import CurrentChat from "./current-chat";

const MemoizedCurrentChat = memo(CurrentChat);

export default function TabActivePanel() {
  const { selectedTab } = useTab();
  const showChat = selectedTab === "chats" || selectedTab === "archived";

  return (
    <div className="relative h-full w-full">
      <section className={`absolute inset-0 bg-black/90 ${showChat ? "visible" : "invisible pointer-events-none"}`}>
        <MemoizedCurrentChat />
      </section>
      {!showChat ? (
        <section className="absolute inset-0 flex flex-col items-center justify-center gap-4 bg-black/90">
          <GearSixIcon className="size-10 text-gray-400" />
          <p className="text-white text-3xl capitalize">{selectedTab}</p>
        </section>
      ) : null}
    </div>
  );
}
