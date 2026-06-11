import { useMemo, useState } from "react";
import { ArrowLeft, Check, CheckCheck, MessageSquarePlus, MoreVertical, Search } from "lucide-react";
import type { AppScreen, Chat } from "../types";

type ChatFilter = "all" | "unread" | "groups";

interface SidebarProps {
  chats: Chat[];
  screen: AppScreen;
  activeChatId: string | null;
  onSelectChat: (id: string) => void;
  onBack: () => void;
}

export default function Sidebar({ chats, screen, activeChatId, onSelectChat, onBack }: SidebarProps) {
  const [searchQuery, setSearchQuery] = useState("");
  const [chatFilter, setChatFilter] = useState<ChatFilter>("all");

  const isArchivedView = screen === "archived";
  const isCommunitiesView = screen === "communities";
  const isStarredView = screen === "starred";
  const showSubfilters = screen === "chats";

  const title = isArchivedView ? "Archived" : isCommunitiesView ? "Communities" : isStarredView ? "Starred" : "Chats";
  const searchPlaceholder = isArchivedView
    ? "Search archived chats"
    : isCommunitiesView
      ? "Search communities"
      : isStarredView
        ? "Search starred chats"
        : "Search or start new chat";
  const helperText = isArchivedView
    ? "Your archived conversations"
    : isCommunitiesView
      ? "Group spaces and shared threads"
      : isStarredView
        ? "Your important conversations"
        : null;

  const visibleChats = useMemo(() => {
    const byScreen = chats.filter((chat) => {
      if (isArchivedView) return chat.isArchived;
      if (isCommunitiesView) return Boolean(chat.isCommunity || chat.isGroup);
      if (isStarredView) return Boolean(chat.isStarred);
      return !chat.isArchived;
    });

    const byFilter = byScreen.filter((chat) => {
      if (!showSubfilters) return true;
      if (chatFilter === "unread") return chat.unreadCount > 0;
      if (chatFilter === "groups") return chat.isGroup;
      return true;
    });

    return byFilter.filter((chat) => {
      const name = chat.isGroup ? chat.name : chat.participants.find((participant) => participant.id !== "me")?.name;
      return name?.toLowerCase().includes(searchQuery.toLowerCase());
    });
  }, [chatFilter, chats, isArchivedView, isCommunitiesView, isStarredView, searchQuery, showSubfilters]);

  return (
    <section className="flex h-full w-[320px] shrink-0 flex-col border-r border-outline-variant/10 bg-surface-container">
      <header className={`flex shrink-0 flex-col justify-end bg-surface-container-high px-4 ${isArchivedView || isCommunitiesView || isStarredView ? "h-[118px]" : "h-[60px]"}`}>
        {isArchivedView || isCommunitiesView || isStarredView ? (
          <div className="flex h-full w-full flex-col pb-2 pt-4">
            <div className="mb-4 flex items-center gap-4">
              <button
                onClick={onBack}
                className="-ml-2 rounded-full p-2 text-on-surface transition-colors active:opacity-80 hover:bg-surface-container-highest"
                type="button"
              >
                <ArrowLeft size={20} />
              </button>
              <h1 className="text-xl font-bold tracking-tight text-on-surface">{title}</h1>
            </div>
          </div>
        ) : (
          <div className="flex w-full items-center justify-between">
            <h1 className="text-xl font-bold tracking-tight text-on-surface">{title}</h1>
            <div className="flex items-center gap-3">
              <button className="rounded-full p-2 text-outline transition-colors hover:bg-surface-container-highest" type="button">
                <MessageSquarePlus size={20} />
              </button>
              <button className="rounded-full p-2 text-outline transition-colors hover:bg-surface-container-highest" type="button">
                <MoreVertical size={20} />
              </button>
            </div>
          </div>
        )}
      </header>

      <div className="bg-surface-container px-3 py-2">
        <div className="relative flex items-center rounded-lg bg-surface-container-high px-3 py-2 ring-primary-container ring-offset-0 focus-within:ring-1">
          <Search className="mr-4 text-outline" size={16} />
          <input
            className="w-full border-none bg-transparent text-sm text-on-surface-variant placeholder:text-outline focus:ring-0"
            placeholder={searchPlaceholder}
            type="text"
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
          />
        </div>

        {showSubfilters ? (
          <div className="no-scrollbar mt-3 flex gap-2 overflow-x-auto pb-1">
            {[
              { key: "all", label: "All" },
              { key: "unread", label: "Unread" },
              { key: "groups", label: "Group chats" },
            ].map((filter) => (
              <button
                key={filter.key}
                type="button"
                onClick={() => setChatFilter(filter.key as ChatFilter)}
                className={`cursor-pointer whitespace-nowrap rounded-full px-3 py-1 text-xs transition-colors ${chatFilter === filter.key ? "bg-surface-container-highest font-medium text-primary" : "bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest"}`}
              >
                {filter.label}
              </button>
            ))}
          </div>
        ) : null}
      </div>

      <div className="flex-1 overflow-y-auto">
        {helperText ? (
          <div className="p-4 text-xs font-medium uppercase tracking-wider text-primary opacity-70">
            {helperText}
          </div>
        ) : null}

        {visibleChats.map((chat) => {
          const participant = chat.participants.find((item) => item.id !== "me");
          const name = chat.isGroup ? chat.name : participant?.name;
          const avatar = chat.isGroup
            ? chat.avatar || "https://images.unsplash.com/photo-1522071823991-b96767a1c56f?q=80&w=100&auto=format&fit=crop"
            : participant?.avatar;
          const isActive = activeChatId === chat.id;

          return (
            <button
              key={chat.id}
              type="button"
              onClick={() => onSelectChat(chat.id)}
              className={`group relative flex w-full cursor-pointer items-center gap-3 px-4 py-3 text-left transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary ${isActive ? "bg-surface-container-highest" : "hover:bg-surface-container-high"}`}
            >
              <div className="relative shrink-0">
                <img className="h-12 w-12 rounded-full object-cover" src={avatar} alt={name} referrerPolicy="no-referrer" />
                {!chat.isGroup && participant?.status === "online" ? (
                  <div className="absolute bottom-0 right-0 h-3 w-3 rounded-full border-2 border-surface-container-low bg-primary" />
                ) : null}
              </div>
              <div className="min-w-0 flex-1">
                <div className="mb-0.5 flex items-baseline justify-between">
                  <h3 className="truncate font-medium text-on-surface">{name}</h3>
                  <span className={`text-[10px] ${chat.unreadCount > 0 ? "font-semibold text-primary" : "text-outline"}`}>
                    {chat.lastMessage?.timestamp}
                  </span>
                </div>
                <div className="flex h-5 items-center justify-between">
                  <div className="flex min-w-0 flex-1 items-center gap-1">
                    {chat.lastMessage?.senderId === "me" ? (
                      chat.lastMessage.status === "read" ? (
                        <CheckCheck size={14} className="shrink-0 text-primary" />
                      ) : (
                        <Check size={14} className="shrink-0 text-outline" />
                      )
                    ) : null}
                    <p className="truncate pr-4 text-sm text-on-surface-variant">{chat.lastMessage?.text}</p>
                  </div>
                  {chat.unreadCount > 0 ? (
                    <div className="flex h-5 min-w-5 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-bold text-on-primary-container">
                      {chat.unreadCount}
                    </div>
                  ) : null}
                </div>
              </div>
            </button>
          );
        })}

        {visibleChats.length === 0 ? (
          <div className="px-8 py-14 text-center text-sm text-on-surface-variant">
            No conversations found in {title.toLowerCase()}.
          </div>
        ) : null}

        {isArchivedView ? (
          <div className="mt-4 border-t border-outline-variant/5 p-8 text-center text-xs leading-relaxed text-outline">
            These chats are archived. They will remain here until you receive a new message or unarchive them manually.
          </div>
        ) : null}
      </div>
    </section>
  );
}
