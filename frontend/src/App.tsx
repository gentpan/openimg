import { useEffect } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider, useAuth } from "./AuthContext";
import { BrandProvider } from "./BrandContext";
import { ToastProvider } from "./ToastContext";
import { UploadProvider } from "./UploadContext";
import UploadPanel from "./components/UploadPanel";
import AdminLayout from "./pages/Admin";
import DashboardPage from "./pages/Dashboard";
import GalleryPage from "./pages/Gallery";
import SharePage from "./pages/Share";
import Home from "./pages/Home";
import LoginPage from "./pages/Login";
import ReferPage from "./pages/Refer";
import RegisterPage from "./pages/Register";
import SettingsPage from "./pages/Settings";
import SpacePage from "./pages/Space";
import UploadPage from "./pages/Upload";

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
    <BrandProvider>
      <ToastProvider>
      <AuthProvider>
        <UploadProvider>
        <BrowserRouter>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route path="/register" element={<RegisterPage />} />
            <Route path="/dashboard" element={<DashboardPage />} />
            <Route path="/upload" element={<UploadPage />} />
            <Route path="/gallery" element={<GalleryPage />} />
            <Route path="/space" element={<SpacePage />} />
            <Route path="/settings" element={<SettingsPage />} />
            <Route path="/refer" element={<ReferPage />} />
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
          {/* Floating upload queue — outside <Routes> so it survives navigation */}
          <UploadPanel />
        </BrowserRouter>
        </UploadProvider>
      </AuthProvider>
      </ToastProvider>
    </BrandProvider>
  );
}

function RequireAdmin({ children }: { children: React.ReactElement }) {
  const { user, loading } = useAuth();
  if (loading) return <CenterMsg>加载中…</CenterMsg>;
  if (!user) return <Navigate to="/login" replace />;
  if (user.role !== "admin") return <CenterMsg>需要管理员权限</CenterMsg>;
  return children;
}

function CenterMsg({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen flex items-center justify-center text-neutral-500">
      {children}
    </div>
  );
}
