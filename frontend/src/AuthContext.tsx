import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { authApi, userApi } from "./api";
import { resetAIStatus } from "./aiStatus";
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

/// 这次页面加载有没有报过时区。放模块级而不是 state:它不参与渲染,
/// 而且 AuthProvider 重挂载时不该重报。
let tzReported = false;

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  async function refresh() {
    try {
      const u = await authApi.me();
      setUser(u);
      // 会话确实建立之后才报时区:未登录时这个接口必然 401。
      // 一次页面加载只报一次——时区在一次会话里不会变,而 refresh() 在签到、
      // 上传之后都会被调,每次都发一个空操作请求没有意义。
      if (!tzReported) {
        tzReported = true;
        void userApi.reportTimezone();
      }
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
          // AI credits are per account, and the cache outlives this session
          // otherwise — the next person to sign in on this tab would see the
          // previous one's remaining count until something refetched it.
          resetAIStatus();
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
