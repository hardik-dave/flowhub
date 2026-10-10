export interface ProductOption {
  code: string
  label: string
}

export const PRODUCTS: ProductOption[] = [
  { code: 'flowos', label: 'FlowOS' },
  { code: 'optionalyzer', label: 'Optionalyzer' },
  { code: 'dhansanketai', label: 'DhanSanket AI' },
  { code: 'pashutrack', label: 'PashuTrack' },
  { code: 'teleflow', label: 'TeleFlow' },
]

export function productLabel(code: string): string {
  return PRODUCTS.find((p) => p.code === code)?.label ?? code
}
