import { UserIcon } from "@phosphor-icons/react";
import Image from "next/image";
import { PropsWithChildren } from "react";

const sizes = {
  6: ["w-6 h-6", 24],
  7: ["w-7 h-7", 28],
  8: ["w-8 h-8", 32],
  10: ["w-10 h-10", 40],
  11: ["w-11 h-11", 44],
  12: ["w-12 h-12", 48],
  24: ["w-24 h-24", 96],
} as const;

function thumbnailUrl(url: string, width: number) {
  try {
    const parsed = new URL(url);
    if (parsed.hostname === "images.unsplash.com") {
      parsed.searchParams.set("w", String(width));
      return parsed.toString();
    }
  } catch {}
  return url;
}

export default function Profile({
  children,
  size,
  url,
}: PropsWithChildren<{ size?: string; url?: string }>) {
  const [sizeClass, pixels] = sizes[Number(size) as keyof typeof sizes] ?? sizes[7];

  const renderAvatar = () => {
    if (url && url.length > 0) {
      return (
        <div
          className={`${sizeClass} bg-amber-300 flex justify-center items-center`}
        >
          <Image
            src={thumbnailUrl(url, pixels * 2)}
            className={sizeClass}
            height={pixels}
            width={pixels}
            alt="profile"
            loading="lazy"
            decoding="async"
          />
        </div>
      );
    }
    return (
      <div className="w-full h-full bg-gray-400 overflow-hidden flex justify-center items-center">
        <UserIcon className={`text-white size-8`} weight="fill" />
      </div>
    );
  };

  return (
    <section
      className={`${sizeClass} overflow-hidden relative flex justify-center items-center rounded-full cursor-pointer`}
    >
      {children ?? renderAvatar()}
    </section>
  );
}
