import { useEffect, Suspense, lazy } from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { useAppStore } from "./store/appStore";
import AccessGatePage from "./pages/AccessGatePage";
import HomePage from "./pages/HomePage";
import JoinPage from "./pages/JoinPage";
import GamePage from "./pages/GamePage";

const AdminPage = lazy(() => import("./pages/AdminPage"));

function RequireAccess({ children }: { children: React.ReactNode }) {
  const { checked, authenticated } = useAppStore();
  if (!checked) return null;
  if (!authenticated) return <AccessGatePage />;
  return <>{children}</>;
}

export default function App() {
  const checkAccess = useAppStore((s) => s.checkAccess);

  useEffect(() => {
    void checkAccess();
  }, [checkAccess]);

  return (
    <BrowserRouter>
      <Routes>
        <Route
          path="/"
          element={
            <RequireAccess>
              <HomePage />
            </RequireAccess>
          }
        />
        <Route
          path="/j/:code"
          element={
            <RequireAccess>
              <JoinPage />
            </RequireAccess>
          }
        />
        <Route
          path="/game/:gameId"
          element={
            <RequireAccess>
              <GamePage />
            </RequireAccess>
          }
        />
        <Route
          path="/admin"
          element={
            <Suspense fallback={null}>
              <AdminPage />
            </Suspense>
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
