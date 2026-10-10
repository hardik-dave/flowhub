import { useState } from 'react'
import { Button } from './ui'

export function OneTimeSecret({
  label,
  value,
  note,
}: {
  label: string
  value: string
  note?: string
}) {
  const [copied, setCopied] = useState(false)

  async function copy() {
    try {
      await navigator.clipboard?.writeText(value)
      setCopied(true)
    } catch {
      setCopied(false)
    }
  }

  return (
    <div className="space-y-2 rounded-md bg-emerald-50 p-3 text-sm">
      <p className="font-medium text-emerald-800">{label} — shown ONCE. Store it now.</p>
      <div className="flex items-center gap-2">
        <code className="flex-1 break-all rounded bg-white px-2 py-1 font-mono text-emerald-900">{value}</code>
        <Button variant="secondary" onClick={() => void copy()}>
          {copied ? 'Copied' : 'Copy'}
        </Button>
      </div>
      {note ? <p className="text-xs text-emerald-700">{note}</p> : null}
    </div>
  )
}
