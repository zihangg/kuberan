/**
 * Format cents as a currency string.
 * @param cents - Amount in cents (e.g., 1050 = $10.50)
 * @param currency - ISO 4217 currency code (default: "USD")
 */
export function formatCurrency(cents: number, currency = "USD"): string {
  const dollars = cents / 100;
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(dollars);
}

/**
 * Format an ISO 8601 date string as a human-readable date.
 * @param iso - ISO 8601 date string
 */
export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

/**
 * Format an ISO 8601 date string as a human-readable date + time.
 * @param iso - ISO 8601 date string
 */
export function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

/**
 * Format a numeric percentage value.
 * @param value - Percentage value (e.g., 65.5 = "65.50%")
 */
export function formatPercentage(value: number): string {
  return `${value.toFixed(2)}%`;
}
