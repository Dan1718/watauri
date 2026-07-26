import {
  DotsThreeVerticalIcon,
  MagnifyingGlassIcon,
  UsersThreeIcon,
  XIcon,
} from "@phosphor-icons/react";
import { memo, useEffect, useRef, useState } from "react";
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
import { createPortal } from "react-dom";

const unreadCountFormatter = new Intl.NumberFormat("en", {
  notation: "compact",
  maximumFractionDigits: 1,
});

const headerMenuItems = [
  { label: "New group", icon: "M500-482q29-32 44.5-73t15.5-85q0-44-15.5-85T500-798q60 8 100 53t40 105q0 60-40 105t-100 53Zm220 322v-120q0-36-16-68.5T662-406q51 18 94.5 46.5T800-280v120h-80Zm80-280v-80h-80v-80h80v-80h80v80h80v80h-80v80h-80Zm-593-87q-47-47-47-113t47-113q47-47 113-47t113 47q47 47 47 113t-47 113q-47 47-113 47t-113-47ZM0-160v-112q0-34 17.5-62.5T64-378q62-31 126-46.5T320-440q66 0 130 15.5T576-378q29 15 46.5 43.5T640-272v112H0Zm320-400q33 0 56.5-23.5T400-640q0-33-23.5-56.5T320-720q-33 0-56.5 23.5T240-640q0 33 23.5 56.5T320-560ZM80-240h480v-32q0-11-5.5-20T540-306q-54-27-109-40.5T320-360q-56 0-111 13.5T100-306q-9 5-14.5 14T80-272v32Zm240-400Zm0 400Z" },
  { label: "Starred messages", icon: "m354-287 126-76 126 77-33-144 111-96-146-13-58-136-58 135-146 13 111 97-33 143ZM233-120l65-281L80-590l288-25 112-265 112 265 288 25-218 189 65 281-247-149-247 149Zm247-350Z" },
  { label: "Select chats", icon: "m424-312 282-282-56-56-226 226-114-114-56 56 170 170ZM200-120q-33 0-56.5-23.5T120-200v-560q0-33 23.5-56.5T200-840h560q33 0 56.5 23.5T840-760v560q0 33-23.5 56.5T760-120H200Zm0-80h560v-560H200v560Zm0-560v560-560Z" },
  { label: "Mark all as read", icon: "M694-160 553-302l56-56 85 85 170-170 56 57-226 226ZM80-80v-720q0-33 23.5-56.5T160-880h640q33 0 56.5 23.5T880-800v280h-80v-280H160v525l46-45h274v80H240L80-80Zm80-240v-480 480Z", dividerAfter: true },
  { label: "App lock", icon: "M240-80q-33 0-56.5-23.5T160-160v-400q0-33 23.5-56.5T240-640h40v-80q0-83 58.5-141.5T480-920q83 0 141.5 58.5T680-720v80h40q33 0 56.5 23.5T800-560v400q0 33-23.5 56.5T720-80H240Zm0-80h480v-400H240v400Zm296.5-143.5Q560-327 560-360t-23.5-56.5Q513-440 480-440t-56.5 23.5Q400-393 400-360t23.5 56.5Q447-280 480-280t56.5-23.5ZM360-640h240v-80q0-50-35-85t-85-35q-50 0-85 35t-35 85v80ZM240-160v-400 400Z" },
  { label: "Log out", icon: "M200-120q-33 0-56.5-23.5T120-200v-560q0-33 23.5-56.5T200-840h280v80H200v560h280v80H200Zm440-160-55-58 102-102H360v-80h327L585-622l55-58 200 200-200 200Z", dangerous: true },
] as const;

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
        {chat.unreadCount > 0 ? (
          <span
            aria-label={`${chat.unreadCount} unread ${chat.unreadCount === 1 ? "message" : "messages"}`}
            className="mt-1 grid place-items-center rounded-full text-[11px] font-semibold leading-4"
            style={{
              backgroundColor: "#21c063",
              color: "#081c15",
              height: 20,
              minWidth: 20,
              paddingInline: chat.unreadCount < 10 ? 0 : 6,
            }}
          >
            {unreadCountFormatter.format(chat.unreadCount).toLowerCase()}
          </span>
        ) : null}
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
  const [isHeaderMenuOpen, setIsHeaderMenuOpen] = useState(false);
  const headerMenuRef = useRef<HTMLDivElement>(null);
  const headerMenuTriggerRef = useRef<HTMLButtonElement>(null);
  const [headerMenuPosition, setHeaderMenuPosition] = useState({ left: 0, top: 0 });

  const updateHeaderMenuPosition = () => {
    const rect = headerMenuTriggerRef.current?.getBoundingClientRect();
    if (!rect) return;

    setHeaderMenuPosition({
      left: rect.left + window.scrollX,
      top: rect.bottom + window.scrollY + 6,
    });
  };

  const toggleHeaderMenu = () => {
    if (!isHeaderMenuOpen) updateHeaderMenuPosition();
    setIsHeaderMenuOpen((open) => !open);
  };

  useEffect(() => {
    if (!isHeaderMenuOpen) return;

    const closeMenu = (event: MouseEvent) => {
      const target = event.target as Node;
      if (headerMenuRef.current?.contains(target) || headerMenuTriggerRef.current?.contains(target)) return;
      setIsHeaderMenuOpen(false);
    };

    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setIsHeaderMenuOpen(false);
    };

    updateHeaderMenuPosition();

    document.addEventListener("mousedown", closeMenu);
    document.addEventListener("keydown", closeOnEscape);
    window.addEventListener("resize", updateHeaderMenuPosition);

    return () => {
      document.removeEventListener("mousedown", closeMenu);
      document.removeEventListener("keydown", closeOnEscape);
      window.removeEventListener("resize", updateHeaderMenuPosition);
    };
  }, [isHeaderMenuOpen]);

  return (
    <section className="w-full h-full flex flex-col gap-3 p-4 relative">
      <section className="w-full flex justify-between items-center">
        <p className="text-white text-2xl font-semibold capitalize">
          {selectedTab}
        </p>
        <section className="flex justify-between items-center gap-2">
          <TooltipWrapper showTooltip={false} onClick={openNewChatWindow}>
            <svg aria-hidden="true" className="size-6 text-white" fill="none" viewBox="0 0 24 24">
              <path fill="currentColor" d="M9.53 13h1.98v1.97c0 .43.25.85.67.98a1 1 0 0 0 1.31-.94v-2.02h1.98c.43 0 .85-.25.98-.67a1 1 0 0 0-.94-1.31h-2.02V9.03c0-.43-.25-.85-.67-.98a1 1 0 0 0-1.31.94v2.02H9.49a1 1 0 0 0-.94 1.31c.13.42.55.67.98.67Z" />
              <path fill="currentColor" fillRule="evenodd" d="M.94 5.53 3 8.85v8.48C3 18.81 4.2 20 5.67 20h13.66c1.48 0 2.67-1.2 2.67-2.67V6.67C22 5.19 20.8 4 19.33 4H1.8a1 1 0 0 0-.85 1.53ZM5 8.28v9.05c0 .37.3.67.67.67h13.66c.37 0 .67-.3.67-.67V6.67c0-.37-.3-.67-.67-.67H3.6L5 8.28Z" clipRule="evenodd" />
            </svg>
          </TooltipWrapper>
          <div className="relative">
            <button
              aria-expanded={isHeaderMenuOpen}
              aria-label="Open chat menu"
              className="flex cursor-pointer items-center justify-center rounded-full bg-transparent p-2 outline-none hover:bg-gray-700/90"
              onClick={toggleHeaderMenu}
              ref={headerMenuTriggerRef}
              type="button"
            >
              <DotsThreeVerticalIcon
                className="size-6 text-white"
                weight="bold"
              />
            </button>
          </div>
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
                  ? "selected-filter bg-green-700/30 text-green-100 border-green-600/30"
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
          chats={filtered}
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
      {typeof document !== "undefined"
        ? createPortal(
          <div
            className="rounded-2xl border border-white/10 p-2 shadow-[0_18px_48px_rgba(0,0,0,0.45)]"
            ref={headerMenuRef}
            role="menu"
            style={{
              backgroundColor: "#161717",
              left: headerMenuPosition.left,
              opacity: isHeaderMenuOpen ? 1 : 0,
              pointerEvents: isHeaderMenuOpen ? "auto" : "none",
              position: "absolute",
              top: headerMenuPosition.top,
              transform: isHeaderMenuOpen ? "scale(0.85)" : "scale(0.72)",
              transformOrigin: "top left",
              transition: `opacity ${isHeaderMenuOpen ? 140 : 70}ms ease-out, transform 180ms cubic-bezier(0.22, 1, 0.36, 1)`,
              zIndex: 9999,
            }}
          >
            {headerMenuItems.map(({ label, icon, dangerous, dividerAfter }) => (
              <div key={label}>
                <button
                  className="my-0.5 flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-left text-[15px] leading-5 whitespace-nowrap transition-colors duration-150"
                  role="menuitem"
                  style={{ color: "#ffffff" }}
                  type="button"
                  onMouseOver={(event) => {
                    event.currentTarget.style.backgroundColor = dangerous ? "#2e1d1f" : "#2e2f2f";
                    event.currentTarget.style.color = dangerous ? "#db8690" : "#ffffff";
                  }}
                  onMouseOut={(event) => {
                    event.currentTarget.style.backgroundColor = "transparent";
                    event.currentTarget.style.color = "#ffffff";
                  }}
                >
                  <svg aria-hidden="true" className="size-5 shrink-0" fill="currentColor" viewBox="0 -960 960 960">
                    <path d={icon} />
                  </svg>
                  <span>{label}</span>
                </button>
                {dividerAfter ? <div className="mx-3 mt-1 mb-2 h-px bg-white/10" /> : null}
              </div>
            ))}
          </div>,
          document.body,
        )
        : null}
    </section>
  );
}
