import { Suspense, lazy, useEffect } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider, useAuth } from "./AuthContext";
import { BrandProvider } from "./BrandContext";
import { DialogProvider } from "./DialogContext";
import { LangProvider, useLang } from "./LangContext";
import { ToastProvider } from "./ToastContext";
import { UploadProvider } from "./UploadContext";
import UploadPanel from "./components/UploadPanel";
import Home from "./pages/Home";

// Home is the landing page, so it stays eager; every other route is a
// dynamic import so the first paint no longer pays for the admin console,
// the chart library, the AI pages and everything else at once.
const AdminLayout = lazy(() => import("./pages/Admin"));
const DashboardPage = lazy(() => import("./pages/Dashboard"));
const DownloadPage = lazy(() => import("./pages/Download"));
const GalleryPage = lazy(() => import("./pages/Gallery"));
const GeneratePage = lazy(() => import("./pages/Generate"));
const SharePage = lazy(() => import("./pages/Share"));
const LoginPage = lazy(() => import("./pages/Login"));
const ReferPage = lazy(() => import("./pages/Refer"));
const DocsPage = lazy(() => import("./pages/Docs"));
const RegisterPage = lazy(() => import("./pages/Register"));
const RetouchPage = lazy(() => import("./pages/Retouch"));
const SettingsPage = lazy(() => import("./pages/Settings"));
const SpacePage = lazy(() => import("./pages/Space"));
const UploadPage = lazy(() => import("./pages/Upload"));

export default function App() {
  useEffect(() => {
    // Capture ?ref=CODE from any landing URL and stash it for the next signup.
    // The cookie is what the backend's OAuth callback reads (it can't see the
    // request body); localStorage is what our JSON register/OTP forms read.
    const params = new URLSearchParams(window.location.search);
    const ref = params.get("ref");
    if (ref && /^[A-Z0-9]{6,16}$/i.test(ref)) {
      const code = ref.toUpperCase();
      try {
        localStorage.setItem("openimg_ref", code);
      } catch {}
      document.cookie = `openimg_ref=${code}; Path=/; Max-Age=${60 * 60 * 24 * 30}; SameSite=Lax`;
    }
  }, []);

  return (
    <LangProvider>
    <BrandProvider>
      <ToastProvider>
      <DialogProvider>
      <AuthProvider>
        <UploadProvider>
        <BrowserRouter>
          <Suspense fallback={<LazyFallback />}>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route path="/register" element={<RegisterPage />} />
            <Route path="/dashboard" element={<DashboardPage />} />
            <Route path="/upload" element={<UploadPage />} />
            <Route path="/gallery" element={<GalleryPage />} />
            {/* Sends itself away when the deployment has no AI key configured;
                see pages/Generate.tsx. */}
            <Route path="/generate" element={<GeneratePage />} />
            {/* Same gate, same reason: no AI key, no route. */}
            <Route path="/retouch" element={<RetouchPage />} />
            <Route path="/space" element={<SpacePage />} />
            <Route path="/settings" element={<SettingsPage />} />
            <Route path="/refer" element={<ReferPage />} />
            <Route path="/docs" element={<DocsPage />} />
            <Route path="/download" element={<DownloadPage />} />
            <Route
              path="/admin/*"
              element={
                <RequireAdmin>
                  <AdminLayout />
                </RequireAdmin>
              }
            />
            {/* Short links. Second to last: every named page above already
              claimed its path, and the reserved-word list on the server keeps
              a code from ever matching one of them. */}
            <Route path="/:code" element={<SharePage />} />
            {/* Home is public — the uploader on it prompts anonymous visitors to
              register rather than hiding the product behind a login wall. */}
            <Route path="/*" element={<Home />} />
          </Routes>
          </Suspense>
          {/* Floating upload queue — outside <Routes> so it survives navigation */}
          <UploadPanel />
        </BrowserRouter>
        </UploadProvider>
      </AuthProvider>
      </DialogProvider>
      </ToastProvider>
    </BrandProvider>
    </LangProvider>
  );
}

// Shown while a lazy route's chunk is still in flight. Deliberately plain:
// it renders on slow networks, so it must not depend on the chunk loading.
function LazyFallback() {
  const { t } = useLang();
  return <CenterMsg>{t.common.loading}</CenterMsg>;
}

function RequireAdmin({ children }: { children: React.ReactElement }) {
  const { t } = useLang();
  const { user, loading } = useAuth();
  if (loading) return <CenterMsg>{t.common.loading}</CenterMsg>;
  if (!user) return <Navigate to="/login" replace />;
  if (user.role !== "admin") return <CenterMsg>{t.common.adminRequired}</CenterMsg>;
  return children;
}

function CenterMsg({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen flex items-center justify-center text-neutral-500">
      {children}
    </div>
  );
}
