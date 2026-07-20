import {
  ChatDotsIcon,
  DotsThreeVerticalIcon,
  UsersThreeIcon,
} from "@phosphor-icons/react";
import TooltipWrapper from "../tooltip-wrapper";
import { useNewChat } from "@/app/hooks/use-new-chat";
import { useChats } from "@/app/hooks/use-chats";
import { Chat, Filters, Message } from "@/app/context/chats-provider";
import Profile from "../profile";
import { useContacts } from "@/app/hooks/use-contacts";
import { useCurrentChat } from "@/app/hooks/use-current-chat";
import { formatTime, getDisplayNameFromJid } from "@/app/utils";
import MessageStatusIcon from "../message-status-icon";

export default function Chats({ selectedTab }: { selectedTab: string }) {
  const { openNewChatWindow } = useNewChat();
  const {
    filter,
    search,
    updateFilter,
    updateSearch,
    chats: { filtered, isLoading, error },
  } = useChats();
  const { getContact } = useContacts();
  const { loadCurrentChat, contact, chatId } = useCurrentChat();

  const getMetaMessage = (
    chat: Chat,
    { message, contactId }: Message
  ): string => {
    if (chat.group) {
      return `${getContact(contactId)?.displayName ?? getDisplayNameFromJid(contactId)}: ${message}`;
    }
    if (contact?.typing && contact.id === contactId) {
      return "typing...";
    }
    return message;
  };

  const renderChat = (chat: Chat) => {
    const currentContact = getContact(
      typeof chat.contactId === "string" ? chat.contactId : ""
    );
    const name =
      typeof chat.contactId === "string"
        ? currentContact?.displayName ?? getDisplayNameFromJid(chat.contactId)
        : chat.groupName ?? getDisplayNameFromJid(chat.id);
    const lastMessage = chat.messages[chat.messages.length - 1];

    return (
      <button
        key={chat.id}
        onClick={() => loadCurrentChat({ chatId: chat.id, page: 0 })}
        className={`outline-none grid grid-cols-6 w-full gap-4 p-2.5 hover:bg-white/10 rounded-xl cursor-pointer ${
          chat.id === chatId
            ? "bg-white/10"
            : ""
        }`}
      >
        <div className="col-span-1">
          {!chat.group ? (
            <Profile size="12" url={currentContact?.contactAvatar} />
          ) : (
            <Profile size="12">
              <div className="h-full w-full flex justify-center items-center bg-white/50">
                <UsersThreeIcon className="size-7 text-white" weight="fill" />
              </div>
            </Profile>
          )}
        </div>
        <div className="col-span-3 flex flex-col justify-center items-start w-full">
          <p className="text-white">{name}</p>
          <div className="flex justify-start items-center gap-1 w-full">
            {lastMessage && <MessageStatusIcon message={lastMessage} />}
            {lastMessage && contact?.typing && contact.id === lastMessage.contactId ? (
              <p className="text-emerald-500 text-sm">
                {getMetaMessage(chat, lastMessage)}
              </p>
            ) : (
              <p
                className={`text-sm ${
                  !lastMessage || chat.read || lastMessage.isSentFromUser
                    ? "text-white/55"
                    : "text-white font-semibold"
                } whitespace-nowrap truncate text-ellipsis overflow-hidden`}
              >
                {lastMessage ? getMetaMessage(chat, lastMessage) : "No messages yet"}
              </p>
            )}
          </div>
        </div>
        <div className="col-span-2 flex flex-col justify-center items-end">
          <p
            className={`text-xs font-semibold ${
              !lastMessage || chat.read || lastMessage.isSentFromUser
                ? "text-white/55"
                : "text-emerald-400"
            }`}
          >
            {lastMessage ? formatTime(lastMessage.timestamp) : ""}
          </p>
        </div>
      </button>
    );
  };

  const renderChats = () => {
    if (isLoading) {
      return (
        <div className="w-full h-full flex justify-center items-center text-white/50">
          Loading...
        </div>
      );
    }

    if (error) {
      return (
        <div className="w-full h-full flex justify-center items-center text-red-300 text-sm text-center px-4">
          {error}
        </div>
      );
    }

    const normalizedSearch = search.toLowerCase();
    const renderedChats = filtered
      .filter((chat) => {
        if (!normalizedSearch) return true;
        const name =
          typeof chat.contactId === "string"
            ? getContact(chat.contactId)?.displayName ?? getDisplayNameFromJid(chat.contactId)
            : chat.groupName ?? getDisplayNameFromJid(chat.id);
        return name?.toLowerCase().includes(normalizedSearch);
      })
      .map(renderChat);

    if (renderedChats.length === 0) {
      return <div className="text-white/50 text-sm px-2 py-4">No chats found</div>;
    }

    return renderedChats;
  };

  return (
    <section className="w-full h-full flex flex-col gap-3 p-4 relative">
      <section className="w-full flex justify-between items-center">
        <p className="text-white text-2xl font-semibold capitalize">
          {selectedTab}
        </p>
        <section className="flex justify-between items-center gap-2">
          <TooltipWrapper showTooltip={false} onClick={openNewChatWindow}>
            <ChatDotsIcon className="text-white size-6" weight="bold" />
          </TooltipWrapper>
          <TooltipWrapper showTooltip={false}>
            <DotsThreeVerticalIcon
              className="text-white size-6"
              weight="bold"
            />
          </TooltipWrapper>
        </section>
      </section>
      <section className="w-full flex flex-col gap-1">
        <input
          className="rounded-full w-full p-2 px-4 outline-none bg-white/10 hover:ring-[1px] hover:ring-gray-600 focus:ring-2 focus:ring-green-500 ring-0 ring-transparent focus:bg-transparent placeholder-gray-400 focus:placeholder-gray-400 text-white"
          placeholder="Search or start a new chat"
          value={search}
          onChange={(event) => updateSearch(event.target.value)}
        />
        <div className="flex justify-start items-center text-white gap-2 mt-2">
          {Object.values(Filters).map((f: string) => (
            <button
              key={f}
              className={`${
                f === filter
                  ? "bg-green-700/30 text-green-100 border-green-600/30"
                  : "border-white/20 hover:bg-white/10"
              } text-sm p-1 px-3 border-[1px] rounded-full cursor-pointer capitalize`}
              onClick={() => updateFilter(f)}
            >
              {f}
            </button>
          ))}
        </div>
      </section>
      <section className="w-full overflow-y-scroll flex flex-col gap-1">
        {renderChats()}
      </section>
    </section>
  );
}
