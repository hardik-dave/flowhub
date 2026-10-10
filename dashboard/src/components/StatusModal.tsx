import { useState } from 'react'
import { Button, ErrorText, Field, Modal, Select, Textarea } from './ui'

export interface StatusOption {
  value: string
  label: string
}

export function StatusModal({
  title,
  initial,
  options,
  onSubmit,
  busy = false,
  error = '',
  onClose,
}: {
  title: string
  initial: string
  options: StatusOption[]
  onSubmit: (status: string, reason: string) => void
  busy?: boolean
  error?: string
  onClose: () => void
}) {
  const [status, setStatus] = useState(initial)
  const [reason, setReason] = useState('')
  const valid = reason.trim().length > 0

  return (
    <Modal title={title} onClose={onClose}>
      <div className="space-y-3">
        <Field label="New status">
          <Select value={status} onChange={(e) => setStatus(e.target.value)}>
            {options.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Reason (required)">
          <Textarea
            rows={3}
            value={reason}
            placeholder="Why is this changing?"
            onChange={(e) => setReason(e.target.value)}
          />
        </Field>
        {error ? <ErrorText>{error}</ErrorText> : null}
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button disabled={!valid || busy} onClick={() => onSubmit(status, reason.trim())}>
            Save
          </Button>
        </div>
      </div>
    </Modal>
  )
}
