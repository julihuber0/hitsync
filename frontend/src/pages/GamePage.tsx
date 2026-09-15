import { useEffect, useRef } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useGameStore } from "../store/gameStore";
import { loadPlayerToken } from "../api/identity";
import Lobby from "../components/Lobby";
import GameBoard from "../components/GameBoard";
import GameOverScreen from "../components/GameOverScreen";

export default function GamePage() {
  const { gameId = "" } = useParams();
  const navigate = useNavigate();
  const connect = useGameStore((s) => s.connect);
  const disconnect = useGameStore((s) => s.disconnect);
  const state = useGameStore((s) => s.state);
  const connected = useGameStore((s) => s.connected);
  const kickedReason = useGameStore((s) => s.kickedReason);
  const disconnectTimer = useRef<number | null>(null);
  const connectedGameID = useRef<string | null>(null);

  useEffect(() => {
    let resumedStrictModeEffect = false;
    if (disconnectTimer.current !== null) {
      window.clearTimeout(disconnectTimer.current);
      disconnectTimer.current = null;
      resumedStrictModeEffect = connectedGameID.current === gameId;
    }
    const token = loadPlayerToken(gameId);
    if (!token) {
      disconnect();
      connectedGameID.current = null;
      navigate("/");
      return;
    }
    if (!resumedStrictModeEffect) {
      connect(token);
      connectedGameID.current = gameId;
    }
    return () => {
      // React Strict Mode intentionally mounts, cleans up, and remounts an
      // effect in development. Deferring this lets the immediate remount keep
      // the socket instead of producing a spurious close-before-open error.
      disconnectTimer.current = window.setTimeout(() => {
        disconnect();
        connectedGameID.current = null;
        disconnectTimer.current = null;
      }, 0);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gameId]);

  useEffect(() => {
    if (kickedReason) navigate("/");
  }, [kickedReason, navigate]);

  if (!state) {
    return (
      <div className="min-h-screen flex items-center justify-center text-neon/50 animate-glow-pulse">
        {connected ? "…" : "Connecting…"}
      </div>
    );
  }

  switch (state.phase) {
    case "LOBBY":
      return <Lobby />;
    case "GAME_OVER":
      return <GameOverScreen />;
    default:
      return <GameBoard />;
  }
}
