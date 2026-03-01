/**
 * Single-quote-only shell quoting.
 *
 * The npm `shell-quote` package falls back to double-quoting for strings that
 * contain single quotes, and inside double quotes it escapes `!` to `\!`.
 * POSIX sh does not strip that backslash (since `!` is not one of the
 * double-quote-special characters), so the `\` leaks into the final output.
 *
 * Single-quoting avoids this entirely: the only character that needs handling
 * inside single quotes is `'` itself, which we embed via the `'\''` idiom
 * (end quote, escaped literal quote, start quote).
 */

/**
 * Shell-quote a single string using single quotes.
 *
 * Every character is preserved literally — no variable expansion, no glob
 * expansion, no history expansion, no command substitution.
 */
export function singleQuote(s: string): string {
  return "'" + s.replace(/'/g, "'\\''") + "'"
}

/**
 * Shell-quote an array of strings and join with spaces.
 *
 * Drop-in replacement for `shellquote.quote(args)` that avoids the
 * double-quoting `!` escaping bug.
 */
export function quoteArgs(args: string[]): string {
  return args.map(singleQuote).join(' ')
}
