import { ChecksIcon } from "@phosphor-icons/react";
import * as Switch from "@radix-ui/react-switch";
import { useProfile } from "@/app/hooks/use-profile";

export default function Settings() {
  const {
    profile: { readReceiptsEnabled },
    setReadReceiptsEnabled,
  } = useProfile();

  return (
    <section className="flex h-full w-full flex-col gap-6 p-4 text-white">
      <h1 className="text-2xl font-semibold">Settings</h1>
      <div>
        <p className="mb-2 px-1 text-xs font-semibold uppercase tracking-wide text-[#21c063]">Privacy</p>
        <div className="flex w-full items-center gap-3 rounded-xl p-3 transition-colors hover:bg-white/10">
          <span className="grid size-10 shrink-0 place-items-center rounded-full bg-white/10 text-white/70">
            <ChecksIcon aria-hidden size={22} weight="bold" />
          </span>
          <div className="min-w-0 flex-1">
            <p className="font-medium">Read receipts</p>
            <p className="text-sm leading-5 text-white/55">Let people know when you have read their messages.</p>
          </div>
          <Switch.Root
            checked={readReceiptsEnabled}
            aria-label="Send read receipts"
            className="relative h-6 w-11 shrink-0 cursor-pointer rounded-full border shadow-inner outline-none transition-colors focus-visible:ring-2 focus-visible:ring-[#21c063] focus-visible:ring-offset-2 focus-visible:ring-offset-[#161717]"
            onCheckedChange={setReadReceiptsEnabled}
            style={{
              backgroundColor: readReceiptsEnabled ? "#21c063" : "#4a4d4c",
              borderColor: readReceiptsEnabled ? "#21c063" : "rgba(255, 255, 255, 0.3)",
            }}
          >
            <Switch.Thumb
              className="block size-5 rounded-full bg-white shadow-md transition-transform duration-200 will-change-transform"
              style={{ transform: `translateX(${readReceiptsEnabled ? 22 : 2}px)` }}
            />
          </Switch.Root>
        </div>
      </div>
    </section>
  );
}
