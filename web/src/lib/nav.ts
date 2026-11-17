/** Only allow same-app relative return paths (no protocol-relative / absolute URLs). */
export function safeNext(next: string | null): string {
  if (!next || !next.startsWith('/') || next.startsWith('//') || next.startsWith('/\\')) return '/'
  if (next.startsWith('/login') || next.startsWith('/setup')) return '/'
  return next
}
