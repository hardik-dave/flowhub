// Money is integer paise everywhere on the wire (AGENTS rule 5). Rupees
// conversion happens ONLY here, at the dashboard UI edge. No floats.

/** Parse a rupee string ("123.45") into integer paise (12345). */
export function rupeesToPaise(input: string | number): number {
  const s = String(input).trim()
  if (s === '') throw new Error('amount is required')
  if (!/^-?\d+(\.\d{1,2})?$/.test(s)) {
    throw new Error('amount must be a number with at most 2 decimals')
  }
  const negative = s.startsWith('-')
  const unsigned = negative ? s.slice(1) : s
  const [whole, frac = ''] = unsigned.split('.')
  const paise = Number(whole) * 100 + Number(frac.padEnd(2, '0'))
  if (!Number.isSafeInteger(paise)) throw new Error('amount is too large')
  return negative ? -paise : paise
}

/** Render integer paise as a fixed 2-decimal rupee string ("123.45"). */
export function paiseToRupees(paise: number): string {
  if (!Number.isInteger(paise)) throw new Error('paise must be an integer')
  const sign = paise < 0 ? '-' : ''
  const abs = Math.abs(paise)
  return `${sign}${Math.floor(abs / 100)}.${String(abs % 100).padStart(2, '0')}`
}

/** Display helper: 12345 -> "₹123.45". */
export function formatINR(paise: number): string {
  return `₹${paiseToRupees(paise)}`
}
