const USERNAME_RE = /^[a-zA-Z0-9._-]{3,32}$/

export function usernameError(u: string): string | null {
  if (!u) return 'Enter a username'
  if (u.length < 3 || u.length > 32) return 'Must be 3–32 characters'
  if (!USERNAME_RE.test(u)) return 'Only letters, numbers, dot, underscore and dash'
  return null
}

export function passwordError(p: string): string | null {
  if (p.length < 8) return 'Use at least 8 characters'
  return null
}
