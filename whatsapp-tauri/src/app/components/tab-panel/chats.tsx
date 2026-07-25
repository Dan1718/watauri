import {
  ChatDotsIcon,
  DotsThreeVerticalIcon,
  MagnifyingGlassIcon,
  UsersThreeIcon,
  XIcon,
} from "@phosphor-icons/react";
import TooltipWrapper from "../tooltip-wrapper";
import { useNewChat } from "@/app/hooks/use-new-chat";
import { useChats } from "@/app/hooks/use-chats";
import { Chat, Filters } from "@/app/context/chats-provider";
import Profile from "../profile";
import { useContacts } from "@/app/hooks/use-contacts";
import { useCurrentChat } from "@/app/hooks/use-current-chat";
import { formatTime, getDisplayNameFromJid } from "@/app/utils";
import MessageStatusIcon from "../message-status-icon";
import { useProfile } from "@/app/hooks/use-profile";
import { memo } from "react";

type ChatRowProps = {
  chat: Chat;
  name: string;
  avatar?: string;
  senderName?: string;
  isCurrent: boolean;
  typingMatchesLastSender: boolean;
  blueTickEnabled: boolean;
  loadCurrentChat: (chat: { chatId: string; page: number; unreadCount: number }) => void;
};

const ChatRow = memo(function ChatRow({
  chat,
  name,
  avatar,
  senderName,
  isCurrent,
  typingMatchesLastSender,
  blueTickEnabled,
  loadCurrentChat,
}: ChatRowProps) {
  const lastMessage = chat.messages[chat.messages.length - 1];
  const metaMessage = lastMessage
    ? !chat.group && typingMatchesLastSender
      ? "typing..."
      : chat.group
        ? `${senderName}: ${lastMessage.message}`
        : lastMessage.message
    : "No messages yet";

  return (
    <button
      onClick={() => loadCurrentChat({
        chatId: chat.id,
        page: 0,
        unreadCount: chat.unreadCount,
      })}
      className={`outline-none flex items-center text-left w-full gap-4 p-2.5 hover:bg-white/10 rounded-xl cursor-pointer ${
        isCurrent ? "bg-white/10" : ""
      }`}
    >
      <div className="shrink-0">
        {!chat.group ? (
          <Profile size="12" url={avatar} />
        ) : (
          <Profile size="12">
            <div className="h-full w-full flex justify-center items-center bg-white/50">
              <UsersThreeIcon className="size-7 text-white" weight="fill" />
            </div>
          </Profile>
        )}
      </div>
      <div className="min-w-0 flex-1 flex flex-col justify-center items-start">
        <p className="text-white break-words whitespace-normal w-full" style={{ textAlign: "left" }}>
          {name}
        </p>
        <div className="flex justify-start items-center gap-1 w-full">
          {lastMessage && (
            <MessageStatusIcon
              isSentFromUser={lastMessage.isSentFromUser}
              read={lastMessage.read}
              delivered={lastMessage.delivered}
              sent={lastMessage.sent}
              pending={lastMessage.pending}
              blueTickEnabled={blueTickEnabled}
            />
          )}
          {lastMessage && typingMatchesLastSender ? (
            <p className="text-emerald-500 text-sm">{metaMessage}</p>
          ) : (
            <p
              className={`text-sm ${
                !lastMessage || chat.read || lastMessage.isSentFromUser
                  ? "text-white/55"
                  : "text-white font-semibold"
              } whitespace-nowrap truncate text-ellipsis overflow-hidden`}
            >
              {metaMessage}
            </p>
          )}
        </div>
      </div>
      <div className="shrink-0 flex flex-col justify-center items-end">
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
});

const ChatList = memo(function ChatList({
  chats,
  search,
  isLoading,
  error,
  getContact,
  currentChatId,
  typingContactId,
  blueTickEnabled,
  loadCurrentChat,
}: {
  chats: Chat[];
  search: string;
  isLoading: boolean;
  error: string | null;
  getContact: ReturnType<typeof useContacts>["getContact"];
  currentChatId: string | null;
  typingContactId?: string;
  blueTickEnabled: boolean;
  loadCurrentChat: ChatRowProps["loadCurrentChat"];
}) {
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
  const renderedChats = chats.flatMap((chat) => {
    const contactId = typeof chat.contactId === "string" ? chat.contactId : undefined;
    const contact = contactId ? getContact(contactId) : undefined;
    const name = contactId
      ? contact?.displayName ?? getDisplayNameFromJid(contactId)
      : chat.groupName ?? getDisplayNameFromJid(chat.id);
    if (normalizedSearch && !name.toLowerCase().includes(normalizedSearch)) return [];

    const lastMessage = chat.messages[chat.messages.length - 1];
    return [
      <ChatRow
        key={chat.id}
        chat={chat}
        name={name}
        avatar={contact?.contactAvatar}
        senderName={lastMessage && chat.group
          ? getContact(lastMessage.contactId)?.displayName ?? getDisplayNameFromJid(lastMessage.contactId)
          : undefined}
        isCurrent={chat.id === currentChatId}
        typingMatchesLastSender={Boolean(lastMessage && typingContactId === lastMessage.contactId)}
        blueTickEnabled={blueTickEnabled}
        loadCurrentChat={loadCurrentChat}
      />,
    ];
  });

  return renderedChats.length > 0
    ? renderedChats
    : <div className="flex h-full items-center justify-center px-4 text-center text-xl text-white">No chats, contacts or messages found</div>;
});

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
  const { profile: { blueTickEnabled } } = useProfile();

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
        <div className="relative">
          <MagnifyingGlassIcon aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 z-10 size-5 -translate-y-1/2 text-white/60" />
          <input
            className="w-full rounded-full border-2 border-transparent bg-[#2e2f2f] p-2 pl-10 pr-10 text-white caret-[#21c063] outline-none placeholder-gray-400 hover:border-gray-600 focus:border-[#21c063] focus:bg-[#161717]"
            placeholder="Search or start a new chat"
            value={search}
            onChange={(event) => updateSearch(event.target.value)}
          />
          {search ? (
            <button aria-label="Clear search" className="absolute right-2 top-1/2 flex -translate-y-1/2 cursor-pointer rounded-full p-1 text-white/80" onClick={() => updateSearch("")} type="button">
              <XIcon className="size-4" weight="bold" />
            </button>
          ) : null}
        </div>
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
      <section className="flex min-h-0 w-full flex-1 flex-col gap-1 overflow-y-auto">
        <ChatList
          chats={filtered.filter((chat) => chat.archived === (selectedTab === "archived"))}
          search={search}
          isLoading={isLoading}
          error={error}
          getContact={getContact}
          currentChatId={chatId}
          typingContactId={contact?.typing ? contact.id : undefined}
          blueTickEnabled={blueTickEnabled}
          loadCurrentChat={loadCurrentChat}
        />
      </section>
    </section>
  );
}
