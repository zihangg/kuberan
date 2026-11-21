/**
 * Day-first date entry for `<DateInput>`.
 *
 * The app's date values are ISO `yyyy-mm-dd` strings (what the API takes and
 * what the native date input produced). The field always shows and accepts
 * `dd/mm/yyyy`, whatever the device or browser locale.
 */

/** Zero-pads a day or month to two digits. */
function pad2(n: number): string {
  return String(n).padStart(2, "0");
}

/** Whether `y-m-d` is a real calendar date (no 31/02, no month 13). */
function isRealDate(y: number, m: number, d: number): boolean {
  if (y < 1000 || y > 9999 || m < 1 || m > 12 || d < 1) return false;
  // Day 0 of the next month is the last day of month m.
  const lastDay = new Date(Date.UTC(y, m, 0)).getUTCDate();
  return d <= lastDay;
}

/** `2026-11-10` -> `10/11/2026`. Anything that is not an ISO date gives "". */
export function isoToDisplay(iso: string): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  if (!m) return "";
  return `${m[3]}/${m[2]}/${m[1]}`;
}

/**
 * Parses what the person typed as day/month/year and returns the ISO date,
 * or null when it is incomplete or not a real date. Accepts `/`, `.`, `-` or
 * spaces as separators with 1-2 digit day and month (`1/5/2026`), or eight
 * bare digits (`01052026`). The year must have four digits.
 */
export function parseDisplayDate(text: string): string | null {
  const t = text.trim();
  let m = /^(\d{1,2})[/.\-\s]+(\d{1,2})[/.\-\s]+(\d{4})$/.exec(t);
  if (!m) m = /^(\d{2})(\d{2})(\d{4})$/.exec(t);
  if (!m) return null;
  const d = Number(m[1]);
  const mo = Number(m[2]);
  const y = Number(m[3]);
  if (!isRealDate(y, mo, d)) return null;
  return `${y}-${pad2(mo)}-${pad2(d)}`;
}

/**
 * Shapes the text while it is typed, as day/month/year. A slash goes in
 * after two digits of day or month (`01052026` -> `01/05/2026`, so a phone's
 * number pad is enough), or wherever the person types a separator themselves
 * (`1/5/2026`, `1.5.2026`). Other characters are dropped and the year stops
 * at four digits.
 */
export function maskDateTyping(raw: string): string {
  let out = "";
  let group = 0; // 0 day, 1 month, 2 year
  let count = 0; // digits in the current group
  for (const ch of raw) {
    if (ch >= "0" && ch <= "9") {
      if (group < 2 && count === 2) {
        out += "/";
        group++;
        count = 0;
      }
      if (group === 2 && count === 4) break;
      out += ch;
      count++;
    } else if (/[/.\-\s]/.test(ch) && group < 2 && count > 0) {
      out += "/";
      group++;
      count = 0;
    }
  }
  return out;
}

/** ISO date -> local `Date` at midnight (for the calendar), or undefined. */
export function isoToLocalDate(iso: string): Date | undefined {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  if (!m) return undefined;
  return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
}

/** Local `Date` -> ISO date, using the calendar day the person picked. */
export function localDateToIso(date: Date): string {
  return `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())}`;
}
