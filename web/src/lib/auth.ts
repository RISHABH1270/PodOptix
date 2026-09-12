// Tiny JWT storage — no libraries needed.
// Token lives in localStorage so page reloads keep the user signed in.

const KEY = 'podoptix.token'
const EMAIL_KEY = 'podoptix.email'

export const auth = {
  getToken(): string | null   { return localStorage.getItem(KEY) },
  getEmail(): string | null   { return localStorage.getItem(EMAIL_KEY) },
  isLoggedIn(): boolean       { return !!localStorage.getItem(KEY) },
  save(token: string, email: string) {
    localStorage.setItem(KEY, token)
    localStorage.setItem(EMAIL_KEY, email)
  },
  clear() {
    localStorage.removeItem(KEY)
    localStorage.removeItem(EMAIL_KEY)
  },
}
