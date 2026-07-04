import dayjs from "dayjs";

export const formatTime = (timestamp: number | string) => {
  if (typeof timestamp === "string") return timestamp;
  return dayjs.unix(timestamp).format("h:mm A");
};

export const getTimestamp = () => {
  const date = new Date();
  const now = dayjs(date).unix();
  return now;
};
