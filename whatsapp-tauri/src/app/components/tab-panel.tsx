import dynamic from "next/dynamic";
import { useNewChat } from "../hooks/use-new-chat";
import TabPanelSwitcher from "./tab-panel/index";

const NewChatWindow = dynamic(() => import("./new-chat"), { ssr: false });

export default function TabPanel() {
  const { isNewChatWindowOpen } = useNewChat();

  return (
    <section className="col-span-7 h-full w-full relative bg-[#161717] border-r-[1px] border-gray-300/20 flex flex-col overflow-hidden">
      <TabPanelSwitcher />
      {isNewChatWindowOpen ? <NewChatWindow /> : null}
    </section>
  );
}
