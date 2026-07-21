import { createContext, PropsWithChildren, useEffect, useState } from "react";
import { getBackendProfile } from "../backend";

export type Profile = {
  id: string;
  name: string;
  blueTickEnabled: boolean;
  avatarUrl: string;
};

export const ProfileContext = createContext<
  | undefined
  | {
      profile: Profile;
      isLoading: boolean;
    }
>(undefined);

const FALLBACK_PROFILE: Profile = {
  id: "me",
  name: "Me",
  blueTickEnabled: true,
  avatarUrl: "",
};

const FALLBACK = {
  profile: FALLBACK_PROFILE,
  isLoading: false,
};

export default function ProfileProvider({ children }: PropsWithChildren) {
  const [value, setValue] = useState({ ...FALLBACK, isLoading: true });

  useEffect(() => {
    const controller = new AbortController();
    getBackendProfile(controller.signal)
      .then(({ id, pushName }) => setValue({
        profile: {
          ...FALLBACK_PROFILE,
          id: id || FALLBACK_PROFILE.id,
          name: pushName || FALLBACK_PROFILE.name,
        },
        isLoading: false,
      }))
      .catch(() => {
        if (!controller.signal.aborted) setValue(FALLBACK);
      });
    return () => controller.abort();
  }, []);

  return (
    <ProfileContext.Provider value={value}>
      {children}
    </ProfileContext.Provider>
  );
}
