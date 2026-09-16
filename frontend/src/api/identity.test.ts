import { describe, it, expect, beforeEach } from "vitest";
import { forgetGame, loadPlayerToken, loadRecentGames, loadStoredGames, saveIdentity } from "./identity";

describe("identity storage", () => {
  beforeEach(() => localStorage.clear());

  it("lists stored games with their player tokens, newest first", () => {
    saveIdentity({ gameId: "g1", playerId: "p1", playerToken: "t1", inviteCode: "AAA111" });
    saveIdentity({ gameId: "g2", playerId: "p2", playerToken: "t2", inviteCode: "BBB222" });
    expect(loadStoredGames().map((g) => [g.gameId, g.playerToken])).toEqual([
      ["g2", "t2"],
      ["g1", "t1"],
    ]);
  });

  it("forgets a game's token and list entry", () => {
    saveIdentity({ gameId: "g1", playerId: "p1", playerToken: "t1", inviteCode: "AAA111" });
    saveIdentity({ gameId: "g2", playerId: "p2", playerToken: "t2", inviteCode: "BBB222" });
    forgetGame("g1");
    expect(loadPlayerToken("g1")).toBeNull();
    expect(loadRecentGames().map((g) => g.gameId)).toEqual(["g2"]);
  });
});
