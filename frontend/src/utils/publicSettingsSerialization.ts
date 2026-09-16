// JSON embedded in an HTML script must not contain a literal closing tag.
// Escaping preserves the decoded data; JSON.stringify alone is not HTML-safe.
export function serializePublicSettings(value: unknown): string {
  return (JSON.stringify(value) ?? 'null').replace(/[<>&\u2028\u2029]/g, (character) =>
    `\\u${character.charCodeAt(0).toString(16).padStart(4, '0')}`,
  )
}
