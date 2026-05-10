// fetchUserData retrieves a user profile by ID.
// Rename this function to loadUserProfile throughout the module.
export function fetchUserData(userId: string): { id: string; name: string } {
  return { id: userId, name: `User ${userId}` };
}

// displayUser calls fetchUserData to render a user card.
export function displayUser(userId: string): string {
  const user = fetchUserData(userId);
  return `Name: ${user.name} (ID: ${user.id})`;
}
