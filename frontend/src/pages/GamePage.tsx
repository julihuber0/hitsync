import { useEffect } from "react";
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

  useEffect(() => {
    const token = loadPlayerToken(gameId);
    if (!token) {
      navigate("/");
      return;
    }
    connect(token);
    return () => disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gameId]);

  useEffect(() => {
    if (kickedReason) navigate("/");
  }, [kickedReason, navigate]);

  if (!state) {
    return (
      <div className="min-h-screen bg-bg flex items-center justify-center text-white/50">
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
