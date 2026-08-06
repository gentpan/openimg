import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { authApi } from "./api";
import type { User } from "./types";

interface AuthCtx {
  user: User | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, code: string, name?: string) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
}

const Ctx = createContext<AuthCtx | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  async function refresh() {
    try {
      const u = await authApi.me();
      setUser(u);
    } catch {
      setUser(null);
    }
  }

  useEffect(() => {
    (async () => {
      await refresh();
      setLoading(false);
    })();
  }, []);

  return (
    <Ctx.Provider
      value={{
        user,
        loading,
        login: async (e, p) => {
          const u = await authApi.login(e, p);
          setUser(u);
        },
        register: async (e, p, code, n) => {
          const ref = readRef();
          const u = await authApi.register(e, p, code, n, ref);
          if (ref) clearRef();
          setUser(u);
        },
        logout: async () => {
          await authApi.logout();
          setUser(null);
        },
        refresh,
      }}
    >
      {children}
    </Ctx.Provider>
  );
}

function readRef(): string | undefined {
  try {
    return localStorage.getItem("openimg_ref") || undefined;
  } catch {
    return undefined;
  }
}
function clearRef() {
  try {
    localStorage.removeItem("openimg_ref");
  } catch {}
  document.cookie = "openimg_ref=; Path=/; Max-Age=0";
}

export function useAuth() {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error("useAuth must be inside AuthProvider");
  return ctx;
}
