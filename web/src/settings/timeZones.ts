/** Every IANA zone the browser knows (none on browsers without Intl.supportedValuesOf). */
export function allTimeZones(): string[] {
  try {
    return Intl.supportedValuesOf("timeZone");
  } catch {
    return [];
  }
}

export function deviceTimeZone(): string | undefined {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || undefined;
  } catch {
    return undefined;
  }
}

/** "America/Port_of_Spain" → "America/Port of Spain". */
export function zoneLabel(zone: string): string {
  return zone.replaceAll("_", " ");
}

/** Zones matching every word of the query, ignoring case, spaces vs underscores and slashes. */
export function searchZones(zones: string[], query: string): string[] {
  const words = query.toLowerCase().split(/[\s/_]+/).filter(Boolean);
  if (words.length === 0) return zones;
  return zones.filter((zone) => {
    const name = zone.toLowerCase();
    return words.every((w) => name.includes(w));
  });
}
