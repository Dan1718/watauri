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
  if (read) {
    return <ChecksIcon className={`${size} ${blueTickEnabled ? "text-blue-400" : "text-white/40"}`} />;
  }
  if (delivered) return <ChecksIcon className={`${size} text-white/40`} />;
  if (sent) return <CheckIcon className={`${size} text-white/40`} />;
  return <WarningCircleIcon className={`${size} text-white/40`} />;
}
