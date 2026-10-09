import { describe, expect, it } from 'vitest'
import { formatINR, paiseToRupees, rupeesToPaise } from './money'

describe('rupeesToPaise', () => {
  it('converts whole rupees', () => {
    expect(rupeesToPaise('123')).toBe(12300)
    expect(rupeesToPaise(5)).toBe(500)
  })

  it('converts paise with 1 or 2 decimals', () => {
    expect(rupeesToPaise('123.4')).toBe(12340)
    expect(rupeesToPaise('123.45')).toBe(12345)
    expect(rupeesToPaise('0.01')).toBe(1)
  })

  it('handles negatives and trims whitespace', () => {
    expect(rupeesToPaise(' -12.50 ')).toBe(-1250)
  })

  it('rejects empty, malformed and over-precise input', () => {
    expect(() => rupeesToPaise('')).toThrow()
    expect(() => rupeesToPaise('12.345')).toThrow()
    expect(() => rupeesToPaise('abc')).toThrow()
    expect(() => rupeesToPaise('1,000')).toThrow()
  })

  it('never returns a non-integer', () => {
    expect(Number.isInteger(rupeesToPaise('999.99'))).toBe(true)
  })
})

describe('paiseToRupees', () => {
  it('renders fixed 2-decimal strings', () => {
    expect(paiseToRupees(12345)).toBe('123.45')
    expect(paiseToRupees(100)).toBe('1.00')
    expect(paiseToRupees(5)).toBe('0.05')
    expect(paiseToRupees(0)).toBe('0.00')
  })

  it('handles negatives', () => {
    expect(paiseToRupees(-1250)).toBe('-12.50')
  })

  it('rejects non-integer paise', () => {
    expect(() => paiseToRupees(1.5)).toThrow()
  })

  it('round-trips through rupeesToPaise', () => {
    for (const paise of [0, 1, 99, 100, 12345, 999999]) {
      expect(rupeesToPaise(paiseToRupees(paise))).toBe(paise)
    }
  })
})

describe('formatINR', () => {
  it('prefixes the rupee sign', () => {
    expect(formatINR(12345)).toBe('₹123.45')
  })
})
