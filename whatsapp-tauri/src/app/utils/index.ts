import dayjs from "dayjs";

export const formatTime = (timestamp: number | string) => {
  if (typeof timestamp === "string") {
    const parsed = dayjs(timestamp);
    return parsed.isValid() ? parsed.format("h:mm A") : timestamp;
  }
  return dayjs.unix(timestamp).format("h:mm A");
};

export const getTimestamp = () => {
  const date = new Date();
  const now = dayjs(date).unix();
  return now;
};

export const getDisplayNameFromJid = (jid: string) => {
  const [user] = jid.split("@");
  return user || jid || "Unknown chat";
};

export const isPhonePlaceholder = (name?: string) =>
  Boolean(name && /^\+?[\d\s∙•*….-]+$/u.test(name));

export const normalizeJid = (jid: string) => jid.replace(/:\d+(?=@)/, "");
