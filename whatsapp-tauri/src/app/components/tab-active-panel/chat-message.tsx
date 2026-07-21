import { Message } from "@/app/context/chats-provider";
import { formatTime } from "@/app/utils";
import MessageStatusIcon from "../message-status-icon";
import Profile from "../profile";

const senderColors = [
  "text-pink-400",
  "text-sky-300",
  "text-teal-300",
  "text-amber-300",
  "text-green-300",
];

function getContactColor(contactId: string) {
  let hash = 0;
  for (const character of contactId) hash = (hash * 31 + character.charCodeAt(0)) >>> 0;
  return senderColors[hash % senderColors.length];
}

export default function ChatMessage({
  message,
  isGroup,
  senderName,
  senderAvatar,
  showSender,
  blueTickEnabled,
}: {
  message: Message;
  isGroup: boolean;
  senderName?: string;
  senderAvatar?: string;
  showSender: boolean;
  blueTickEnabled: boolean;
}) {
  const bubble = (
    <div
      className={`min-w-0 rounded-lg px-2 py-1.5 ${
        message.isSentFromUser ? "bg-emerald-900" : "bg-[#30383d]"
      }`}
    >
      {isGroup && !message.isSentFromUser && showSender ? (
        <p className={`mb-1 text-xs font-semibold ${getContactColor(message.contactId)}`}>
          {senderName}
        </p>
      ) : null}
      <div className="flex min-w-0 max-w-full items-end justify-between gap-2">
        <p className="min-w-0 whitespace-pre-wrap break-words text-sm text-white">
          {message.message}
        </p>
        <p className="shrink-0 text-xs text-white/80">{formatTime(message.timestamp)}</p>
        <MessageStatusIcon
          isSentFromUser={message.isSentFromUser}
          read={message.read}
          delivered={message.delivered}
          sent={message.sent}
          blueTickEnabled={blueTickEnabled}
          isInMessage
        />
      </div>
    </div>
  );

  if (!isGroup || message.isSentFromUser) return bubble;

  return (
    <div className="flex min-w-0 max-w-full items-start gap-2">
      <div className="h-7 w-7 shrink-0">
        {showSender ? <Profile url={senderAvatar} /> : null}
      </div>
      {bubble}
    </div>
  );
}
