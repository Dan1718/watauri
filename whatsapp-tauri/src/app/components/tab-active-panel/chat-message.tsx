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

const supportedTlds = new Set([
  "ai", "app", "asia", "au", "biz", "blog", "ca", "cc", "cloud", "co", "com", "dev", "edu",
  "gov", "in", "info", "int", "io", "me", "mil", "mobi", "name", "net", "online", "org", "pro",
  "site", "tech", "tel", "travel", "tv", "uk", "us", "xyz",
]);
const messageUrlPattern = /(\bhttps?:\/\/[^\s<]+[^\s<.,:;"')\]}]|(?<![@\w.-])\b(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}(?::\d{1,5})?(?:[/?#][^\s<]*[^\s<.,:;"')\]}])?)/gi;

export function splitMessageText(message: string) {
  return message.split(messageUrlPattern);
}

export type MentionTarget = {
  jid: string;
  name?: string;
};

export type MessageContentPart =
  | { type: "text"; text: string }
  | { type: "mention"; target: MentionTarget; fallbackText: string };

export function splitMessageContent(message: string, mentionTargets?: MentionTarget[]): MessageContentPart[] {
  let mentionIndex = 0;
  const parts = splitMessageText(message).flatMap((part, partIndex): MessageContentPart[] => {
    if (partIndex % 2 !== 0 || !mentionTargets?.length) return [{ type: "text", text: part }];
    return part.split(/(@[+\d*•∙.…-]+)/gu).map((segment, segmentIndex) => {
      if (segmentIndex % 2 === 0) return { type: "text", text: segment };
      const target = mentionTargets[mentionIndex++];
      return target
        ? { type: "mention", target, fallbackText: segment }
        : { type: "text", text: segment };
    });
  });
  return parts.filter((part) => part.type !== "text" || part.text.length > 0);
}

export function getMessageHref(text: string) {
  if (/^https?:\/\//i.test(text)) return text;
  const tld = text.split(/[/:?#]/)[0].split(".").pop()?.toLowerCase();
  return tld && supportedTlds.has(tld) ? `https://${text}` : null;
}

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
  mentionTargets,
  onOpenMention,
  showSender,
  blueTickEnabled,
}: {
  message: Message;
  isGroup: boolean;
  senderName?: string;
  senderAvatar?: string;
  mentionTargets?: MentionTarget[];
  onOpenMention: (jid: string) => void;
  showSender: boolean;
  blueTickEnabled: boolean;
}) {
  const textParts = splitMessageContent(message.message, mentionTargets);
  const bubble = (
    <div
      data-unread-message-id={!message.isSentFromUser && !message.read ? message.id : undefined}
      className={`min-w-0 rounded-lg px-2 py-1.5 ${
        message.isSentFromUser ? "bg-[#144D37]" : "bg-[#242626]"
      }`}
    >
      {isGroup && !message.isSentFromUser && showSender ? (
        <p className={`mb-1 text-xs font-semibold ${getContactColor(message.contactId)}`}>
          {senderName}
        </p>
      ) : null}
      <div className="flex min-w-0 max-w-full items-end justify-between gap-2">
        <p className="min-w-0 whitespace-pre-wrap break-words text-sm text-white">
          {textParts.map((part, index) => {
            if (part.type === "mention") {
              return (
                <button
                  aria-label={`Open chat with ${part.target.name ?? part.fallbackText}`}
                  className="cursor-pointer text-[#53bdeb] hover:underline"
                  key={index}
                  onClick={() => onOpenMention(part.target.jid)}
                  style={{ color: "#53bdeb" }}
                  type="button"
                >
                  @{part.target.name ?? part.fallbackText.slice(1)}
                </button>
              );
            }
            const href = getMessageHref(part.text);
            return href ? (
              <a
                className="hover:opacity-80"
                href={href}
                key={index}
                rel="noreferrer"
                target="_blank"
              >
                <span style={{ borderBottom: "1px solid currentColor", paddingBottom: "1px" }}>
                  {part.text}
                  <span
                    aria-hidden="true"
                    className="material-symbols-outlined ml-0.5 align-middle !text-[14px]"
                    style={{ fontVariationSettings: '"FILL" 0, "wght" 600, "GRAD" 0, "opsz" 24' }}
                  >
                    arrow_outward
                  </span>
                </span>
                <span className="sr-only"> (opens in a new tab)</span>
              </a>
            ) : part.text;
          })}
        </p>
        <p className="shrink-0 text-xs text-white/80">{formatTime(message.timestamp)}</p>
        <MessageStatusIcon
          isSentFromUser={message.isSentFromUser}
          read={message.read}
          delivered={message.delivered}
          sent={message.sent}
          pending={message.pending}
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
