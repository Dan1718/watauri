import { Chat, Message } from "@/app/context/chats-provider";
import { Contact } from "@/app/context/contacts-provider";
import { CurrentChatContactsGroup } from "@/app/context/current-chat-provider";
import { useChats } from "@/app/hooks/use-chats";
import { getDisplayNameFromJid } from "@/app/utils";
import { XIcon } from "@phosphor-icons/react";
import { useEffect, useState } from "react";
import Profile from "../profile";

const durations = ["Off", "1 day", "7 days", "3 months"];
const mediaFilters = [
  ["image", "Images"],
  ["video", "Videos"],
  ["audio", "Audio"],
  ["link", "Links"],
  ["file", "Files"],
] as const;

function Icon({ children }: { children: string }) {
  return <span aria-hidden="true" className="material-symbols-outlined">{children}</span>;
}

function latestMessageTime(chat: Chat) {
  const timestamp = chat.messages.at(-1)?.timestamp;
  if (typeof timestamp === "number") return timestamp;
  return timestamp ? Date.parse(timestamp) || 0 : 0;
}

function orderGroups(groups: Chat[], limit = groups.length) {
  if (limit >= groups.length) {
    return [...groups].sort((a, b) => latestMessageTime(b) - latestMessageTime(a));
  }
  const ordered: Chat[] = [];
  for (const group of groups) {
    const time = latestMessageTime(group);
    const index = ordered.findIndex((item) => latestMessageTime(item) < time);
    ordered.splice(index < 0 ? ordered.length : index, 0, group);
    if (ordered.length > limit) ordered.pop();
  }
  return ordered;
}

function GroupRow({ chat }: { chat: Chat }) {
  return (
    <div className="flex items-center gap-3 px-1 py-2.5">
      <Profile size="11" url={chat.groupAvatar}>
        {!chat.groupAvatar ? <div className="flex h-full w-full items-center justify-center bg-white/10"><Icon>group</Icon></div> : undefined}
      </Profile>
      <span className="min-w-0 flex-1 truncate text-sm text-white/85">{chat.groupName ?? getDisplayNameFromJid(chat.id)}</span>
    </div>
  );
}

export default function ChatInfoPanel({ chatId, contact, group, messages, userId }: {
  chatId: string;
  contact: Contact | null;
  group: CurrentChatContactsGroup | null;
  messages: Message[];
  userId: string;
}) {
  const { chats: { complete }, setChatArchived } = useChats();
  const chat = complete.find((chat) => chat.id === chatId);
  const [muted, setMuted] = useState(() => Boolean(chat?.muted));
  const [pinned, setPinned] = useState(() => Boolean(chat?.pinned));
  const archived = Boolean(chat?.archived);
  const [showAll, setShowAll] = useState(false);
  const [mediaFilter, setMediaFilter] = useState<(typeof mediaFilters)[number][0]>("image");
  const [duration, setDuration] = useState("Off");
  const [durationOpen, setDurationOpen] = useState(false);
  const [overflowOpen, setOverflowOpen] = useState(false);
  const [notes, setNotes] = useState("");
  const [memberSearch, setMemberSearch] = useState("");
  const [showAllMembers, setShowAllMembers] = useState(false);
  const [commonGroupsOpen, setCommonGroupsOpen] = useState(false);
  const [commonGroupSearch, setCommonGroupSearch] = useState("");
  const [{ all: commonGroups, preview: commonGroupPreview }] = useState(() => {
    const directContactId = complete.find((chat) => chat.id === chatId && !chat.group)?.contactId;
    const all = typeof directContactId === "string"
      ? complete.filter((chat) => chat.group && Array.isArray(chat.contactId) && chat.contactId.includes(directContactId))
      : [];
    return { all, preview: orderGroups(all, 5) };
  });
  const [orderedCommonGroups, setOrderedCommonGroups] = useState<Chat[] | null>(null);
  const members = (group ? Object.entries(group.contacts) : [])
    .map(([id, contact]) => ({
      contact,
      id,
      isSelf: id === userId || contact?.id === userId,
      name: contact?.displayName || (contact?.phone ? `+${contact.phone}` : getDisplayNameFromJid(id)),
    }))
    .sort((a, b) => Number(Boolean(b.contact?.isSaved)) - Number(Boolean(a.contact?.isSaved)) ||
      Number(b.isSelf) - Number(a.isSelf) ||
      a.name.localeCompare(b.name));
  const search = memberSearch.trim().toLowerCase();
  const matchingMembers = members.filter(({ contact, id, isSelf, name }) =>
    [name, id, contact?.phone, contact?.statusMessage, isSelf ? "You" : ""].some((value) => value?.toLowerCase().includes(search))
  );
  const visibleMembers = showAllMembers ? matchingMembers : matchingMembers.slice(0, 5);
  const media = messages.filter((message) =>
    message.mediaType || /https?:\/\/\S+/i.test(message.message)
  );
  const mediaCategory = (message: Message) => {
    if (!message.mediaType && /https?:\/\/\S+/i.test(message.message)) return "link";
    if (["image", "video", "audio", "link"].includes(message.mediaType ?? "")) return message.mediaType;
    return "file";
  };
  const visibleMedia = showAll
    ? media.filter((message) => mediaCategory(message) === mediaFilter)
    : media.slice(0, 4);

  useEffect(() => {
    setNotes(localStorage.getItem(`chat-notes:${chatId}`) ?? "");
  }, [chatId]);

  const actionClass = (active: boolean) =>
    `chat-info-action flex size-11 items-center justify-center rounded-full border transition-colors focus-visible:outline-2 focus-visible:outline-emerald-400 ${active ? "border-emerald-400/60 bg-emerald-400 text-[#07130d]" : "border-white/10 bg-white/7 text-white/75 hover:bg-white/12"}`;

  const toggleMedia = () => {
    if (!showAll) setMediaFilter("image");
    setShowAll(!showAll);
  };

  const openCommonGroups = () => {
    setOrderedCommonGroups(orderGroups(commonGroups));
    setCommonGroupsOpen(true);
  };

  const visibleCommonGroups = (orderedCommonGroups ?? []).filter((chat) =>
    (chat.groupName ?? getDisplayNameFromJid(chat.id)).toLowerCase().includes(commonGroupSearch.toLowerCase())
  );

  return (
    <aside aria-label={group ? "Group info" : "Contact info"} className="relative flex h-full w-[min(366px,40%)] min-w-72 shrink-0 flex-col overflow-hidden border-l border-white/10 bg-[#161717] text-white [&_button]:cursor-pointer">
      <div className="min-h-0 flex-1 overflow-y-auto px-5 pb-8">
        <section className="relative flex flex-col items-center pb-3 pt-8 text-center">
          {group ? <button className="green-action absolute right-0 top-5 rounded px-1 text-sm font-medium text-emerald-400 focus-visible:outline-2 focus-visible:outline-emerald-400" type="button">Edit</button> : null}
          <div className="relative mb-4 flex w-full justify-center" style={{ paddingBottom: 24 }}>
            <div className="rounded-full ring-1 ring-white/10">
              <Profile size="24" url={group?.avatar || contact?.contactAvatar}>
                {group && !group.avatar ? <div className="flex h-full w-full items-center justify-center bg-white/15"><Icon>group</Icon></div> : undefined}
              </Profile>
            </div>
            <button aria-label={muted ? "Unmute chat" : "Mute chat"} aria-pressed={muted} className={`${actionClass(muted)} absolute`} onClick={() => setMuted((value) => !value)} style={{ bottom: 0, left: 0 }} type="button">
              <Icon>{muted ? "notifications_off" : "notifications"}</Icon>
            </button>
            <button aria-label={pinned ? "Unpin chat" : "Pin chat"} aria-pressed={pinned} className={`${actionClass(pinned)} absolute`} onClick={() => setPinned((value) => !value)} style={{ bottom: 0, right: 0 }} type="button">
              <Icon>{pinned ? "keep_off" : "push_pin"}</Icon>
            </button>
          </div>
          <h3 className="max-w-full truncate text-xl font-semibold">{group?.name ?? contact?.displayName}</h3>
          {group ? <button className="green-action mt-1 rounded px-1 text-sm text-emerald-400 focus-visible:outline-2 focus-visible:outline-emerald-400" type="button">Add a description</button> : <p className="mt-1 text-sm text-white/50">{contact?.typing ? "typing..." : contact?.statusMessage || "Online"}</p>}
        </section>

        {group ? (
            <section className="py-3">
              <div className="mb-3 flex items-center justify-between py-1" style={showAllMembers ? { background: "#161717", position: "sticky", top: 0, zIndex: 20 } : undefined}>
                <h3 className="text-sm font-medium text-white/70">Members</h3>
                {showAllMembers ? <button className="green-action w-20 rounded text-sm font-medium leading-5 text-emerald-400 focus-visible:outline-2 focus-visible:outline-emerald-400" onClick={() => setShowAllMembers(false)} type="button">Show less</button> : null}
              </div>
              <div className="overflow-hidden rounded-xl border border-white/10 bg-white/3">
                <div className="flex items-center gap-2 px-3 py-2.5 text-white/50 focus-within:text-emerald-400">
                  <Icon>search</Icon>
                  <input aria-label="Search group members" className="min-w-0 flex-1 bg-transparent text-sm text-white outline-none placeholder:text-white/40 [&::-webkit-search-cancel-button]:hidden" onChange={(event) => setMemberSearch(event.target.value)} placeholder={`Search ${members.length} group members`} type="search" value={memberSearch} />
                  {memberSearch ? <button aria-label="Clear search" className="flex rounded-full p-1 text-white/80" onClick={() => setMemberSearch("")} type="button"><XIcon className="size-4" weight="bold" /></button> : null}
                </div>
                <div className="px-2">
                  <button className="flex w-full items-center gap-3 rounded-lg px-2 py-2 text-left text-sm font-medium text-emerald-400 transition-colors hover:bg-white/10 focus-visible:outline-2 focus-visible:outline-emerald-400" type="button"><span className="grid size-8 place-items-center rounded-full bg-emerald-400/15"><Icon>group_add</Icon></span>Add members</button>
                </div>
                <div className="px-2 pb-2">
                  {visibleMembers.map(({ contact: member, id, isSelf, name }) => (
                    <div className="flex min-w-0 items-center gap-3 rounded-lg px-2 py-2 transition-colors hover:bg-white/10" key={id}>
                      <Profile size="8" url={member?.contactAvatar} />
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <span className="truncate text-sm text-white/90">{name}</span>
                          {isSelf ? <span className="rounded-full bg-emerald-400/12 px-2 py-0.5 text-[10px] font-medium text-emerald-400">You</span> : null}
                        </div>
                        {member?.statusMessage ? <p className="mt-0.5 truncate text-xs text-white/45">{member.statusMessage}</p> : null}
                      </div>
                    </div>
                  ))}
                  {visibleMembers.length === 0 ? <p className="py-5 text-center text-sm text-white/35">{memberSearch ? "No members found" : "No member data available"}</p> : null}
                  {matchingMembers.length > 5 && !showAllMembers ? <button className="green-action flex w-full items-center justify-center gap-1 rounded-lg py-2.5 text-sm font-medium text-emerald-400 focus-visible:outline-2 focus-visible:outline-emerald-400" onClick={() => setShowAllMembers(true)} type="button">Show all <Icon>expand_more</Icon></button> : null}
                </div>
              </div>
            </section>
        ) : null}

        {media.length > 0 ? (
          <section className="pb-3 pt-3">
            <div className="mb-3 flex items-center justify-between py-1" style={showAll ? { background: "#161717", position: "sticky", top: 0, zIndex: 20 } : undefined}>
              <h3 className="text-sm font-medium text-white/70">Media</h3>
              <button className="green-action w-20 rounded text-sm font-medium leading-5 text-emerald-400 focus-visible:outline-2 focus-visible:outline-emerald-400" onClick={toggleMedia} type="button">{showAll ? "Show less" : "Show all"}</button>
            </div>
            <div className={showAll ? "rounded-xl bg-white/5 p-2" : ""}>
              {showAll ? (
                <div aria-label="Filter media" className="relative mb-2 grid grid-cols-5 rounded-xl bg-black/60 p-1">
                  <span
                    aria-hidden="true"
                    className="absolute bottom-1 left-1 top-1 w-[calc((100%-0.5rem)/5)] rounded-lg bg-white/15 transition-transform duration-200 ease-out motion-reduce:transition-none"
                    style={{ transform: `translateX(${mediaFilters.findIndex(([value]) => value === mediaFilter) * 100}%)` }}
                  />
                  {mediaFilters.map(([value, label]) => (
                    <button
                      aria-pressed={mediaFilter === value}
                      className={`media-filter-action relative z-10 rounded-lg px-1 py-1.5 text-xs transition-colors ${mediaFilter === value ? "text-white" : "text-white/60 hover:text-white"}`}
                      key={value}
                      onClick={() => setMediaFilter(value)}
                      type="button"
                    >
                      {label}
                    </button>
                  ))}
                </div>
              ) : null}
              {visibleMedia.length > 0 ? (
                <div className="grid grid-cols-2 gap-2">
                  {visibleMedia.map((message) => {
                    const type = message.mediaType || "link";
                    return (
                      <div className="flex aspect-[4/3] min-w-0 flex-col items-center justify-center gap-2 rounded-xl border border-white/8 bg-[#1d1e1e] p-3 text-center text-white/55" key={message.id}>
                        <Icon>{type === "image" ? "image" : type === "video" ? "movie" : type === "audio" ? "audio_file" : type === "document" ? "description" : "link"}</Icon>
                        <span className="max-w-full truncate text-xs capitalize">{type}</span>
                      </div>
                    );
                  })}
                </div>
              ) : (
                <p className="py-6 text-center text-sm text-white/35">Nothing here</p>
              )}
            </div>
          </section>
        ) : null}

        <section className="pb-3 pt-3">
          <label className="mb-2 block text-sm font-medium text-white/70">Disappearing messages</label>
          <div className="relative">
            <button aria-expanded={durationOpen} className="flex w-full items-center justify-between rounded-xl border border-white/10 bg-[#1d1e1e] px-4 py-3 text-left text-sm focus-visible:outline-2 focus-visible:outline-emerald-400" onClick={() => setDurationOpen((value) => !value)} style={durationOpen ? { borderBottomLeftRadius: 0, borderBottomRightRadius: 0, borderBottomWidth: 0 } : undefined} type="button">
              {duration}<Icon>{durationOpen ? "expand_less" : "expand_more"}</Icon>
            </button>
            {durationOpen ? (
              <div className="overflow-hidden border border-white/10 bg-[#1d1e1e] py-1 shadow-2xl" role="menu" style={{ borderRadius: "0 0 0.75rem 0.75rem", borderTopWidth: 0, left: 0, position: "absolute", right: 0, top: "100%", zIndex: 30 }}>
                {durations.map((option) => (
                  <button aria-checked={duration === option} className="flex w-full items-center justify-between px-4 py-2.5 text-left text-sm text-white/80 hover:bg-white/8 focus:bg-white/8 focus:outline-none" key={option} onClick={() => { setDuration(option); setDurationOpen(false); }} role="menuitemradio" type="button">
                    {option}{duration === option ? <Icon>check</Icon> : null}
                  </button>
                ))}
              </div>
            ) : null}
          </div>
        </section>

        <section className="pb-6 pt-3">
          <textarea
            aria-label="Notes"
            className="notes-textarea min-h-32 w-full overflow-hidden rounded-xl border border-white/10 bg-white/5 p-4 text-sm leading-6 caret-emerald-400 outline-none placeholder:text-white/30 focus:border-emerald-400/70"
            id={`notes-${chatId}`}
            onChange={(event) => { setNotes(event.target.value); localStorage.setItem(`chat-notes:${chatId}`, event.target.value); }}
            placeholder="Notes"
            style={{ fieldSizing: "content", resize: "none" }}
            value={notes}
          />
        </section>

        {!group && commonGroups.length > 0 ? (
          <section className="pb-3">
            <h3 className="mb-2 text-sm font-medium text-white/70">{commonGroups.length} {commonGroups.length === 1 ? "group" : "groups"} in common</h3>
            <div>
              {commonGroupPreview.map((chat) => <GroupRow chat={chat} key={chat.id} />)}
            </div>
            {commonGroups.length > 5 ? (
              <button className="flex w-full items-center justify-between rounded-lg px-1 py-2 text-sm text-white/70 hover:bg-white/5 hover:text-white focus-visible:outline-2 focus-visible:outline-emerald-400" onClick={openCommonGroups} type="button">
                <span>Show more</span><Icon>chevron_right</Icon>
              </button>
            ) : null}
          </section>
        ) : null}
      </div>

      <nav aria-label="Chat actions" className="relative grid shrink-0 grid-cols-4 bg-[#161717] px-2 py-2">
        <button aria-pressed={archived} className={`flex min-w-0 flex-col items-center gap-1 rounded-lg py-2 text-[11px] hover:bg-white/8 focus-visible:outline-2 focus-visible:outline-emerald-400 ${archived ? "text-emerald-400" : "text-white/60 hover:text-white"}`} onClick={() => setChatArchived(chatId, !archived)} type="button">
          <Icon>{archived ? "unarchive" : "archive"}</Icon><span className="truncate">{archived ? "Unarchive" : "Archive"}</span>
        </button>
        {[{ icon: "schedule", label: "Remind me" }, { icon: "search", label: "Search" }].map((action) => (
          <button className="flex min-w-0 flex-col items-center gap-1 rounded-lg py-2 text-[11px] text-white/60 hover:bg-white/8 hover:text-white focus-visible:outline-2 focus-visible:outline-emerald-400" key={action.label} type="button">
            <Icon>{action.icon}</Icon><span className="truncate">{action.label}</span>
          </button>
        ))}
        <button aria-expanded={overflowOpen} aria-label="More chat actions" className="flex flex-col items-center gap-1 rounded-lg py-2 text-[11px] text-white/60 hover:bg-white/8 hover:text-white focus-visible:outline-2 focus-visible:outline-emerald-400" onClick={() => setOverflowOpen((value) => !value)} type="button">
          <Icon>more_horiz</Icon><span>More</span>
        </button>
        {overflowOpen ? (
          <div className="absolute bottom-[68px] right-3 w-48 overflow-hidden rounded-xl border border-white/10 bg-[#1d1e1e] py-1 shadow-2xl">
            {(group ? [["logout", "Exit group"], ["delete", "Delete group"]] : [["block", "Block contact"], ["delete", "Delete chat"]]).map(([icon, label]) => (
              <button className="flex w-full items-center gap-3 px-4 py-3 text-left text-sm text-[#ff8a8a]" key={label} type="button"><Icon>{icon}</Icon>{label}</button>
            ))}
          </div>
        ) : null}
      </nav>

      <section
        aria-hidden={!commonGroupsOpen}
        aria-label="Groups in common"
        className={`absolute inset-0 z-40 flex flex-col bg-[#161717] transition-transform duration-300 ease-out motion-reduce:transition-none ${commonGroupsOpen ? "translate-x-0" : "translate-x-full pointer-events-none"}`}
        inert={!commonGroupsOpen}
      >
        <header className="flex shrink-0 items-center gap-3 border-b border-white/10 px-4 py-4">
          <button aria-label="Back to contact info" className="flex size-9 items-center justify-center rounded-full text-white/75 hover:bg-white/10 hover:text-white focus-visible:outline-2 focus-visible:outline-emerald-400" onClick={() => setCommonGroupsOpen(false)} type="button"><Icon>arrow_back</Icon></button>
          <h2 className="text-base font-semibold">Groups in common</h2>
        </header>
        <div className="shrink-0 px-4 pb-3 pt-4">
          <label className="flex items-center gap-2 rounded-full bg-white/10 px-4 py-2.5 text-white/50 focus-within:ring-1 focus-within:ring-emerald-400/70">
            <Icon>search</Icon>
            <input aria-label="Search groups in common" className="min-w-0 flex-1 bg-transparent text-sm text-white outline-none placeholder:text-white/40" onChange={(event) => setCommonGroupSearch(event.target.value)} placeholder="Search groups" type="search" value={commonGroupSearch} />
          </label>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-4 pb-6">
          {visibleCommonGroups.map((chat) => <GroupRow chat={chat} key={chat.id} />)}
          {visibleCommonGroups.length === 0 ? <p className="py-10 text-center text-sm text-white/40">No groups found</p> : null}
        </div>
      </section>
    </aside>
  );
}
