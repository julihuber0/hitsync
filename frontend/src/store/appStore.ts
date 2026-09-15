import { create } from "zustand";
import { api, type AppConfig } from "../api/client";

interface AppStore {
  checked: boolean;
  authenticated: boolean;
  config: AppConfig | null;
  checkAccess: () => Promise<void>;
  setAuthenticated: (config: AppConfig) => void;
  logout: () => Promise<void>;
}

export const useAppStore = create<AppStore>((set) => ({
  checked: false,
  authenticated: false,
  config: null,

  checkAccess: async () => {
    try {
      const { authenticated } = await api.accessStatus();
      if (!authenticated) {
        set({ authenticated: false, config: null, checked: true });
        return;
      }
      const config = await api.config();
      set({ authenticated: true, config, checked: true });
    } catch {
      set({ authenticated: false, config: null, checked: true });
    }
  },

  setAuthenticated: (config) => set({ authenticated: true, config }),

  logout: async () => {
    await api.logout().catch(() => undefined);
    set({ authenticated: false, config: null });
  },
}));
