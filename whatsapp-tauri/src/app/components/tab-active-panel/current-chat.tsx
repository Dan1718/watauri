import { FormEvent, useState } from "react";
import { Message } from "@/app/context/chats-provider";
import { useCurrentChat } from "@/app/hooks/use-current-chat";
import Reaction from "../message/reaction";
import ContactHeader from "./contact-header";
import ChatMessage from "./chat-message";
import MessageReactions from "./message-reactions";

export default function CurrentChat() {
  const { chatId, group, messages, isLoading, error, isSending, sendMessage } = useCurrentChat();
  const [messageText, setMessageText] = useState("");

  if (!chatId) {
    return (
      <section className="w-full h-full text-white flex justify-center items-center">
        Please select a chat to see messages
      </section>
    );
  }

  const getMessageSpacing = (
    index: number,
    reactionsCount?: number
  ): string => {
    if (index === messages.length - 1) {
      return "mb-0";
    } else if (reactionsCount && reactionsCount > 0) {
      return "mb-4";
    } else if (
      messages[index].isSentFromUser === messages[index + 1]?.isSentFromUser &&
      messages[index].contactId === messages[index + 1]?.contactId
    ) {
      return "mb-0.5";
    }
    return "mb-4";
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const text = messageText.trim();
    if (!text || isSending) return;
    const sent = await sendMessage(text);
    if (sent) setMessageText("");
  };

  return (
    <section className="w-full h-full min-h-0 flex flex-col">
      <ContactHeader />
      <div className="min-h-0 w-full flex flex-1 flex-col justify-end items-center relative">
        <div className="absolute background-custom h-full w-full"></div>

        <section
          className="p-4 pb-0 w-full min-h-0 flex flex-1 flex-col items-center relative overflow-y-auto"
          style={{ justifyContent: "safe flex-end" }}
        >
          <div className="w-full flex justify-center items-center">
            <div className="rounded-full overflow-hidden bg-black z-20 w-fit">
              <p className="bg-white/20 text-white/55 h-full w-full text-xs p-1 px-2">
                Today
              </p>
            </div>
          </div>
          {error && <div className="text-red-300 text-sm py-2">{error}</div>}
          {isLoading && <div className="text-white">Loading...</div>}
          {messages.map((message: Message, index: number) => (
            <div
              className={`w-full flex items-center ${
                message.isSentFromUser ? "justify-end" : "justify-start"
              }`}
              key={index}
            >
              <div
                className={`flex min-w-0 gap-2 items-center ${
                  group ? "w-[60%]" : "max-w-full"
                } ${message.isSentFromUser ? "justify-end" : "justify-start"} ${getMessageSpacing(
                  index,
                  message.reactions?.length
                )} relative`}
              >
                {message.isSentFromUser && <Reaction isSentFromUser={true} />}
                <ChatMessage message={message} />
                {!message.isSentFromUser && <Reaction isSentFromUser={false} />}
                {message.reactions?.length && (
                  <MessageReactions
                    reactions={message.reactions}
                    isSentFromUser={message.isSentFromUser}
                  />
                )}
              </div>
            </div>
          ))}
        </section>

        <form className="w-full z-50 h-auto p-4" onSubmit={handleSubmit}>
          <div className="bg-black rounded-full overflow-hidden">
            <div className="bg-white/15 rounded-full">
              <input
                className="w-full outline-none p-3 px-4 text-white placeholder-white/60 caret-green-400 text-sm rounded-full"
                disabled={isSending}
                placeholder={isSending ? "Sending..." : "Type a message"}
                value={messageText}
                onChange={(event) => setMessageText(event.target.value)}
              />
            </div>
          </div>
        </form>
      </div>
    </section>
  );
}
