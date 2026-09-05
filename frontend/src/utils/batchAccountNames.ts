export function parseMultilineValues(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map(item => item.trim())
    .filter(Boolean)
}

export function buildBatchAccountNames(
  baseName: string,
  count: number,
  existingNames: Iterable<string> = []
): string[] {
  const usedNames = new Set(Array.from(existingNames, name => name.trim()).filter(Boolean))
  const names: string[] = []
  let nextIndex = 1

  while (names.length < count) {
    const candidate = `${baseName.trim()}-${nextIndex}`
    nextIndex += 1
    if (usedNames.has(candidate)) continue
    usedNames.add(candidate)
    names.push(candidate)
  }

  return names
}
