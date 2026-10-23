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

export const resolveContactDisplayName = (options: {
  name?: string;
  pushName?: string;
  phoneNumber?: string;
  jid: string;
}) => {
  if (options.name && !isPhonePlaceholder(options.name)) return options.name;
  if (options.pushName && !isPhonePlaceholder(options.pushName)) return options.pushName;
  const phoneNumber = options.phoneNumber || (/^\d+@s\.whatsapp\.net$/u.test(options.jid)
    ? options.jid.slice(0, options.jid.indexOf("@"))
    : undefined);
  if (phoneNumber) return `+${phoneNumber.replace(/^\+/, "")}`;
  return "Unknown contact";
};

export const normalizeJid = (jid: string) => jid.replace(/:\d+(?=@)/, "");
