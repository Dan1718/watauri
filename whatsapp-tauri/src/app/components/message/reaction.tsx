import { PlusCircleIcon, SmileyIcon } from "@phosphor-icons/react";

const reactions = ["👍🏼", "❤️", "😂", "😮", "🥲", "🙏🏻"];

export default function Reaction({
  isSentFromUser,
  isOpen,
  messageId,
  onToggle,
}: {
  isSentFromUser: boolean;
  isOpen: boolean;
  messageId: string;
  onToggle: (messageId: string) => void;
}) {
  return (
    <div
      className={`relative flex items-center justify-center transition-opacity group-hover/message:opacity-100 focus-within:opacity-100 ${
        isOpen ? "opacity-100" : "opacity-0"
      }`}
    >
      {isOpen ? (
        <div
          className={`absolute -top-16 z-50 flex items-center gap-2 rounded-full bg-[#252d32] px-4 py-2 ${
            isSentFromUser ? "right-0" : "left-0"
          }`}
        >
          {reactions.map((reaction) => (
            <button
              key={reaction}
              type="button"
              aria-label={`React with ${reaction}`}
              className="cursor-pointer text-3xl"
            >
              {reaction}
            </button>
          ))}
          <button type="button" aria-label="More reactions" className="cursor-pointer text-white">
            <PlusCircleIcon weight="duotone" className="size-8" />
          </button>
        </div>
      ) : null}
      <button
        type="button"
        aria-label="Add reaction"
        aria-expanded={isOpen}
        className="cursor-pointer text-white/40"
        onClick={() => onToggle(messageId)}
      >
        <SmileyIcon weight="regular" className="size-5" />
      </button>
    </div>
  );
}
