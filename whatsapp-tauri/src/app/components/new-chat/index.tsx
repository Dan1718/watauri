import { useNewChat } from "@/app/hooks/use-new-chat";
import {
  ArrowLeftIcon,
  MagnifyingGlassIcon,
  UserPlusIcon,
  UsersIcon,
  UsersThreeIcon,
  XIcon,
} from "@phosphor-icons/react";
import TooltipWrapper from "../tooltip-wrapper";
import Profile from "../profile";
import { useContacts } from "@/app/hooks/use-contacts";
import { Contact } from "@/app/context/contacts-provider";
import { useEffect, useRef, useState } from "react";

const CONTACT_PAGE_SIZE = 50;

export default function NewChatWindow() {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const { closeNewChatWindow } = useNewChat();
  const { dictionary, filterContacts, search } = useContacts();
  const [visibleContactCount, setVisibleContactCount] = useState(CONTACT_PAGE_SIZE);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const contactCount = dictionary.reduce(
    (count, [, contacts]) => count + contacts.length,
    0,
  );
  let remainingContacts = visibleContactCount;
  const visibleDictionary = dictionary.flatMap(([letter, contacts]) => {
    const visibleContacts = contacts.slice(0, remainingContacts);
    remainingContacts -= visibleContacts.length;
    return visibleContacts.length ? [[letter, visibleContacts] as [string, Contact[]]] : [];
  });

  const renderDictionary = (entries: [string, Contact[]], index: number) => {
    return (
      <section key={index} className="pt-4 px-4 w-full flex flex-col gap-1">
        {entries[1].length >= 1 && (
          <p className="font-semibold text-white/50 pl-4 mb-4">{entries[0]}</p>
        )}
        {entries[1].map((contact: Contact) => (
          <div
            key={contact.id}
            className="flex w-full justify-start items-center gap-4 p-2.5 hover:bg-white/10 rounded-xl cursor-pointer"
          >
            <Profile size="12" url={contact.contactAvatar} />
            <div className="flex flex-col justify-center items-start">
              <p className="text-white">{contact.displayName}</p>
              <p className="text-white/55">{contact.statusMessage}</p>
            </div>
          </div>
        ))}
      </section>
    );
  };

  return (
    <section className="absolute inset-0 z-10 h-full w-full bg-white">
      <section className="w-full h-full bg-black/90 flex flex-col">
        <section className="flex justify-start items-center gap-2 pt-4 px-4 w-full">
              <TooltipWrapper onClick={closeNewChatWindow} showTooltip={false}>
                <ArrowLeftIcon className="size-6 text-white" />
              </TooltipWrapper>
              <p className="text-white ml-2">New Chat</p>
        </section>
        <section className="pt-4 px-4 w-full">
          <div className="relative">
            <MagnifyingGlassIcon aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 z-10 size-5 -translate-y-1/2 text-white/60" />
              <input
                className="w-full rounded-full border-2 border-transparent bg-[#2e2f2f] p-2 pl-10 pr-10 text-white caret-[#21c063] outline-none placeholder-gray-400 hover:border-gray-600 focus:border-[#21c063] focus:bg-[#161717]"
                placeholder="Search for name or number"
                onChange={(event) => {
                  setVisibleContactCount(CONTACT_PAGE_SIZE);
                  filterContacts(event.target.value);
                }}
                ref={inputRef}
                value={search}
              />
              {search ? (
                <button aria-label="Clear search" className="absolute right-2 top-1/2 flex -translate-y-1/2 cursor-pointer rounded-full p-1 text-white/80" onClick={() => {
                  setVisibleContactCount(CONTACT_PAGE_SIZE);
                  filterContacts("");
                }} type="button">
                  <XIcon className="size-4" weight="bold" />
                </button>
              ) : null}
          </div>
        </section>
        <section
          className="w-full h-full overflow-y-scroll scrollbar-hide pb-4"
          onScroll={(event) => {
            const { clientHeight, scrollHeight, scrollTop } = event.currentTarget;
            if (scrollTop + clientHeight < scrollHeight - 200) return;
            setVisibleContactCount((count) => Math.min(count + CONTACT_PAGE_SIZE, contactCount));
          }}
        >
              {search.length === 0 && (
                <section className="pt-4 px-4 w-full flex flex-col gap-1">
                  <div className="flex w-full justify-start items-center gap-4 p-2.5 hover:bg-white/10 rounded-xl cursor-pointer">
                    <div className="size-12 rounded-full bg-green-500 flex justify-center items-center">
                      <UsersIcon className="text-white size-6" weight="fill" />
                    </div>
                    <div className="flex flex-col justify-center items-start">
                      <p className="text-white font-semibold">New Group</p>
                    </div>
                  </div>
                  <div className="flex w-full justify-start items-center gap-4 p-2.5 hover:bg-white/10 rounded-xl cursor-pointer">
                    <div className="size-12 rounded-full bg-green-500 flex justify-center items-center">
                      <UserPlusIcon
                        className="text-white size-6"
                        weight="fill"
                      />
                    </div>
                    <div className="flex flex-col justify-center items-start">
                      <p className="text-white font-semibold">New Contact</p>
                    </div>
                  </div>
                  <div className="flex w-full justify-start items-center gap-4 p-2.5 hover:bg-white/10 rounded-xl cursor-pointer">
                    <div className="size-12 rounded-full bg-green-500 flex justify-center items-center">
                      <UsersThreeIcon
                        className="text-white size-6"
                        weight="fill"
                      />
                    </div>
                    <div className="flex flex-col justify-center items-start">
                      <p className="text-white font-semibold">New Community</p>
                    </div>
                  </div>
                </section>
              )}
              {visibleDictionary.map(renderDictionary)}
        </section>
      </section>
    </section>
  );
}
