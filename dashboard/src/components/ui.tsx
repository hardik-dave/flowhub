import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, SelectHTMLAttributes, TextareaHTMLAttributes } from 'react'

type Variant = 'primary' | 'secondary' | 'danger' | 'ghost'

const variantClasses: Record<Variant, string> = {
  primary: 'bg-indigo-600 text-white hover:bg-indigo-500 disabled:bg-indigo-300',
  secondary: 'bg-white text-slate-700 border border-slate-300 hover:bg-slate-50 disabled:text-slate-400',
  danger: 'bg-red-600 text-white hover:bg-red-500 disabled:bg-red-300',
  ghost: 'bg-transparent text-slate-600 hover:bg-slate-100',
}

export function Button({
  variant = 'primary',
  className = '',
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant }) {
  return (
    <button
      {...props}
      className={`inline-flex items-center justify-center rounded-md px-3 py-1.5 text-sm font-medium transition disabled:cursor-not-allowed ${variantClasses[variant]} ${className}`}
    />
  )
}

export function Card({
  title,
  children,
  className = '',
}: {
  title?: string
  children: ReactNode
  className?: string
}) {
  return (
    <div className={`rounded-lg border border-slate-200 bg-white p-4 shadow-sm ${className}`}>
      {title ? (
        <h2 className="mb-3 text-base font-semibold text-slate-800">{title}</h2>
      ) : null}
      {children}
    </div>
  )
}

export function Field({
  label,
  hint,
  children,
}: {
  label: string
  hint?: string
  children: ReactNode
}) {
  return (
    <label className="block space-y-1">
      <span className="block text-sm font-medium text-slate-700">{label}</span>
      {children}
      {hint ? <span className="block text-xs text-slate-500">{hint}</span> : null}
    </label>
  )
}

const inputClasses =
  'block w-full rounded-md border border-slate-300 px-2.5 py-1.5 text-sm shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500'

export function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={`${inputClasses} ${props.className ?? ''}`} />
}

export function Select(props: SelectHTMLAttributes<HTMLSelectElement>) {
  return <select {...props} className={`${inputClasses} ${props.className ?? ''}`} />
}

export function Textarea(props: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea {...props} className={`${inputClasses} ${props.className ?? ''}`} />
}

const statusColors: Record<string, string> = {
  ACTIVE: 'bg-emerald-100 text-emerald-800',
  PENDING_VERIFICATION: 'bg-amber-100 text-amber-800',
  DEACTIVATED: 'bg-slate-200 text-slate-700',
  BANNED: 'bg-red-100 text-red-800',
  DISABLED: 'bg-red-100 text-red-800',
  INACTIVE: 'bg-slate-200 text-slate-700',
  valid: 'bg-emerald-100 text-emerald-800',
  paused: 'bg-amber-100 text-amber-800',
  revoked: 'bg-red-100 text-red-800',
  not_found: 'bg-slate-200 text-slate-700',
}

export function Badge({ value, className = '' }: { value: string; className?: string }) {
  const color = statusColors[value] ?? 'bg-slate-100 text-slate-700'
  return (
    <span className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${color} ${className}`}>
      {value.replace(/_/g, ' ')}
    </span>
  )
}

export function ErrorText({ children }: { children?: ReactNode }) {
  if (!children) return null
  return <p className="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{children}</p>
}

export function Modal({
  title,
  onClose,
  children,
  wide = false,
}: {
  title: string
  onClose: () => void
  children: ReactNode
  wide?: boolean
}) {
  return (
    <div className="fixed inset-0 z-40 flex items-start justify-center overflow-y-auto bg-slate-900/40 p-4 sm:p-8">
      <div
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className={`w-full rounded-lg bg-white shadow-xl ${wide ? 'max-w-3xl' : 'max-w-lg'}`}
      >
        <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3">
          <h2 className="text-base font-semibold text-slate-800">{title}</h2>
          <button
            onClick={onClose}
            aria-label="Close"
            className="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
          >
            ✕
          </button>
        </div>
        <div className="p-4">{children}</div>
      </div>
    </div>
  )
}

export function Spinner({ label = 'Loading…' }: { label?: string }) {
  return <p className="py-8 text-center text-sm text-slate-500">{label}</p>
}
