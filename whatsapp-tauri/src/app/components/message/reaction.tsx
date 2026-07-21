import { SmileyIcon } from "@phosphor-icons/react";

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
          id={`reaction-menu-${messageId}`}
          className={`absolute -top-16 z-50 flex items-center gap-2 rounded-full bg-[#252d32] px-4 py-2 ${
            isSentFromUser ? "right-0" : "left-0"
          }`}
        >
          <span className="whitespace-nowrap text-sm text-white/70">Reactions are not available yet</span>
        </div>
      ) : null}
      <button
        type="button"
        aria-label={isOpen ? "Close reaction menu" : "View reaction options"}
        aria-controls={`reaction-menu-${messageId}`}
        aria-expanded={isOpen}
        className="cursor-pointer text-white/40"
        onClick={() => onToggle(messageId)}
      >
        <SmileyIcon weight="regular" className="size-5" />
      </button>
    </div>
  );
}
