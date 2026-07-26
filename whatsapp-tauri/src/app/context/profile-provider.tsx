import { createContext, PropsWithChildren, useEffect, useState } from "react";
import { getBackendProfile } from "../backend";

export type Profile = {
  id: string;
  name: string;
  blueTickEnabled: boolean;
  readReceiptsEnabled: boolean;
  avatarUrl: string;
};

export const ProfileContext = createContext<
  | undefined
  | {
      profile: Profile;
      isLoading: boolean;
      setReadReceiptsEnabled: (enabled: boolean) => void;
    }
>(undefined);

const READ_RECEIPTS_KEY = "read-receipts-enabled";

const FALLBACK_PROFILE: Profile = {
  id: "me",
  name: "Me",
  blueTickEnabled: true,
  readReceiptsEnabled: true,
  avatarUrl: "",
};

const FALLBACK = {
  profile: FALLBACK_PROFILE,
  isLoading: false,
};

export default function ProfileProvider({ children }: PropsWithChildren) {
  const [value, setValue] = useState({ ...FALLBACK, isLoading: true });

  const setReadReceiptsEnabled = (enabled: boolean) => {
    localStorage.setItem(READ_RECEIPTS_KEY, String(enabled));
    setValue((current) => ({
      ...current,
      profile: { ...current.profile, readReceiptsEnabled: enabled },
    }));
  };

  useEffect(() => {
    const stored = localStorage.getItem(READ_RECEIPTS_KEY);
    if (stored !== null) {
      setValue((current) => ({
        ...current,
        profile: { ...current.profile, readReceiptsEnabled: stored === "true" },
      }));
    }

    const controller = new AbortController();
    getBackendProfile(controller.signal)
      .then(({ id, pushName }) => setValue((current) => ({
        profile: {
          ...current.profile,
          id: id || FALLBACK_PROFILE.id,
          name: pushName || FALLBACK_PROFILE.name,
        },
        isLoading: false,
      })))
      .catch(() => {
        if (!controller.signal.aborted) setValue((current) => ({ ...current, isLoading: false }));
      });
    return () => controller.abort();
  }, []);

  return (
    <ProfileContext.Provider value={{ ...value, setReadReceiptsEnabled }}>
      {children}
    </ProfileContext.Provider>
  );
}
