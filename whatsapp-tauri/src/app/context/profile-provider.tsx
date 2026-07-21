import { createContext, PropsWithChildren } from "react";

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

const PROFILE = {
  profile: {
    id: "me",
    name: "Me",
    blueTickEnabled: true,
    avatarUrl: "",
  },
  isLoading: false,
};

export default function ProfileProvider({ children }: PropsWithChildren) {
  return (
    <ProfileContext.Provider value={PROFILE}>
      {children}
    </ProfileContext.Provider>
  );
}
