import { createContext, PropsWithChildren, useEffect, useState } from "react";
import { listBackendChats } from "../backend";

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

export default function ProfileProvider({ children }: PropsWithChildren) {
  const [profile, setProfile] = useState<{
    profile: Profile;
    isLoading: boolean;
  }>({
    profile: {
      id: "",
      name: "",
      blueTickEnabled: false,
      avatarUrl: "",
    },
    isLoading: false,
  });

  useEffect(() => {
    const fetchProfile = async () => {
      setProfile((prev) => ({ ...prev, isLoading: true }));
      const chats = await listBackendChats();
      const user = chats
        .flatMap((chat) => chat.participants)
        .find((participant) => participant.id === "me");
      const data = {
        id: user?.id ?? "me",
        name: user?.name ?? "Me",
        blueTickEnabled: true,
        avatarUrl: user?.avatar ?? "",
      };
      setProfile((prev) => ({
        ...prev,
        profile: data,
        isLoading: false,
      }));
    };

    fetchProfile();
  }, []);

  return (
    <ProfileContext.Provider value={{ ...profile }}>
      {children}
    </ProfileContext.Provider>
  );
}
