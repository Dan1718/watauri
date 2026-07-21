"use client";

import { useRef, type PointerEvent } from "react";
import ChatsProvider from "../context/chats-provider";
import ContactsProvider from "../context/contacts-provider";
import CurrentChatProvider from "../context/current-chat-provider";
import NewChatProvider from "../context/new-chat-provider";
import ProfileProvider from "../context/profile-provider";
import TabProvider from "../context/tab-provider";
import TabActivePanel from "./tab-active-panel";
import TabIcons from "./tab-icons";
import TabPanel from "./tab-panel";

type ResizeGeometry = {
  element: HTMLDivElement;
  left: number;
  max: number;
};

export default function AuthenticatedShell() {
  const chatListRef = useRef<HTMLDivElement>(null);
  const resizeRef = useRef<ResizeGeometry | null>(null);

  const stopResize = (event: PointerEvent<HTMLDivElement>) => {
    resizeRef.current = null;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  };

  return (
    <ProfileProvider>
      <TabProvider>
        <ContactsProvider>
          <ChatsProvider>
            <CurrentChatProvider>
              <NewChatProvider>
                <section className="flex h-full w-full overflow-hidden">
                  <div className="h-full w-20 shrink-0">
                    <TabIcons />
                  </div>
                  <div
                    className="relative h-full shrink-0"
                    ref={chatListRef}
                    style={{
                      width: "clamp(320px, calc(100vw - 500px), 560px)",
                      minWidth: 320,
                      maxWidth: "min(560px, calc(100vw - 500px))",
                    }}
                  >
                    <TabPanel />
                    <div
                      aria-orientation="vertical"
                      className="absolute -right-1 top-0 z-50 h-full w-2 cursor-col-resize bg-transparent transition-colors hover:bg-emerald-400/20 active:bg-emerald-400/30"
                      onPointerCancel={stopResize}
                      onPointerDown={(event) => {
                        const element = chatListRef.current;
                        if (!element) return;
                        event.preventDefault();
                        event.currentTarget.setPointerCapture(event.pointerId);
                        const { left } = element.getBoundingClientRect();
                        resizeRef.current = {
                          element,
                          left,
                          max: Math.max(320, window.innerWidth - left - 420),
                        };
                      }}
                      onPointerMove={(event) => {
                        const resize = resizeRef.current;
                        if (!resize) return;
                        resize.element.style.width = `${Math.min(Math.max(event.clientX - resize.left, 320), resize.max)}px`;
                      }}
                      onPointerUp={stopResize}
                      role="separator"
                    />
                  </div>
                  <div className="min-w-0 flex-1">
                    <TabActivePanel />
                  </div>
                </section>
              </NewChatProvider>
            </CurrentChatProvider>
          </ChatsProvider>
        </ContactsProvider>
      </TabProvider>
    </ProfileProvider>
  );
}
