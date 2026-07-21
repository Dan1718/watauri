import { CheckIcon, ChecksIcon, WarningCircleIcon } from "@phosphor-icons/react";

export default function MessageStatusIcon({
  isSentFromUser,
  read,
  delivered,
  sent,
  blueTickEnabled,
  isInMessage,
}: {
  isSentFromUser: boolean;
  read?: boolean;
  delivered?: boolean;
  sent?: boolean;
  blueTickEnabled: boolean;
  isInMessage?: boolean;
}) {
  if (!isSentFromUser) return null;

  const size = isInMessage ? "size-4" : "size-5";
  const label = read ? "Read" : delivered ? "Delivered" : sent ? "Sent" : "Failed to send";
  const icon = read
    ? <ChecksIcon aria-hidden className={`${size} ${blueTickEnabled ? "text-blue-400" : "text-white/40"}`} />
    : delivered
      ? <ChecksIcon aria-hidden className={`${size} text-white/40`} />
      : sent
        ? <CheckIcon aria-hidden className={`${size} text-white/40`} />
        : <WarningCircleIcon aria-hidden className={`${size} text-white/40`} />;
  return <span aria-label={label} role="img">{icon}</span>;
}
