export function canPresentMembershipLevel(hasCurrentLevel: boolean, currentRank: number, targetRank: number): boolean {
  return !hasCurrentLevel || targetRank > currentRank;
}

