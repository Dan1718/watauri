import { CheckIcon, ChecksIcon, WarningCircleIcon } from "@phosphor-icons/react";

export default function MessageStatusIcon({
  isSentFromUser,
  read,
  delivered,
  sent,
  pending,
  blueTickEnabled,
  isInMessage,
}: {
  isSentFromUser: boolean;
  read?: boolean;
  delivered?: boolean;
  sent?: boolean;
  pending?: boolean;
  blueTickEnabled: boolean;
  isInMessage?: boolean;
}) {
  if (!isSentFromUser) return null;

  const size = isInMessage ? "size-4" : "size-5";
  const label = pending ? "Sending" : read ? "Read" : delivered ? "Delivered" : sent ? "Sent" : "Failed to send";
  const icon = pending
    ? (
      <svg aria-hidden className={`${size} shrink-0 text-white/40`} viewBox="0 0 100 100" fill="none">
        <rect x="22" y="22" width="56" height="56" rx="20" stroke="currentColor" strokeWidth="8" />
        <path d="M49 37v15h15" stroke="currentColor" strokeWidth="8" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    )
    : read
    ? <ChecksIcon aria-hidden className={`${size} ${blueTickEnabled ? "text-blue-400" : "text-white/40"}`} />
    : delivered
      ? <ChecksIcon aria-hidden className={`${size} text-white/40`} />
      : sent
        ? <CheckIcon aria-hidden className={`${size} text-white/40`} />
        : <WarningCircleIcon aria-hidden className={`${size} text-white/40`} />;
  return <span aria-label={label} role="img">{icon}</span>;
}
