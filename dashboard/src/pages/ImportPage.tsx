import { useState, type FormEvent } from 'react'
import { useMutation } from '@tanstack/react-query'
import { ApiError, dash } from '../api/client'
import { Button, Card, ErrorText, Field, Input } from '../components/ui'
import type { ImportResult } from '../api/types'

function downloadTemplate() {
  const header = 'username,first_name,last_name,mobile,email,city,state,broker_client_code,referral_code,product_code,plan,paid_at,valid_from'
  const blob = new Blob([header + '\n'], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'hub_import_template.csv'
  a.click()
  URL.revokeObjectURL(url)
}

export function ImportPage() {
  const [file, setFile] = useState<File | null>(null)
  const [result, setResult] = useState<ImportResult | null>(null)
  const [error, setError] = useState('')

  const importCsv = useMutation({
    mutationFn: (f: File) => dash.importCsv(f),
    onSuccess: (d) => {
      setResult(d)
      setError('')
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Import failed.')
    },
  })

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (!file) return
    importCsv.mutate(file)
  }

  function downloadResults() {
    if (!result) return
    const lines = ['row,username,password,license_key']
    for (const c of result.created) {
      lines.push([c.row, c.username, c.password, c.license_key].join(','))
    }
    const blob = new Blob([lines.join('\n')], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `hub_import_results_${result.batch_id}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold text-slate-800">CSV import</h1>
          <p className="text-sm text-slate-500">Upload users per SPEC §8. Per-row failures do not abort the batch.</p>
        </div>
        <Button variant="secondary" onClick={downloadTemplate}>Download template</Button>
      </div>

      <Card>
        <form className="flex flex-wrap items-center gap-3" onSubmit={onSubmit}>
          <Field label="CSV file">
            <Input type="file" accept=".csv,text/csv" onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
          </Field>
          <Button type="submit" disabled={!file || importCsv.isPending}>Upload</Button>
        </form>
        {error ? <ErrorText>{error}</ErrorText> : null}
      </Card>

      {result && (
        <Card>
          <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
            <div>
              <h2 className="text-lg font-semibold text-slate-800">Batch #{result.batch_id}</h2>
              <p className="text-sm text-slate-500">{result.filename} · {result.row_count} rows · OK {result.ok_count} · Errors {result.error_count}</p>
            </div>
            {result.ok_count > 0 && <Button variant="secondary" onClick={downloadResults}>Download credentials CSV</Button>}
          </div>

          {result.errors.length > 0 && (
            <div className="mb-4">
              <h3 className="mb-2 text-sm font-semibold text-slate-700">Row errors</h3>
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-slate-200 text-sm">
                  <thead className="bg-slate-50">
                    <tr>
                      <th className="px-3 py-2 text-left">Row</th>
                      <th className="px-3 py-2 text-left">Reason</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100 bg-white">
                    {result.errors.map((e) => (
                      <tr key={`${e.row}-${e.reason}`}>
                        <td className="px-3 py-2 text-slate-600">{e.row}</td>
                        <td className="px-3 py-2 text-slate-600">{e.reason}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {result.created.length > 0 && (
            <div>
              <h3 className="mb-2 text-sm font-semibold text-slate-700">Created (secrets shown ONCE)</h3>
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-slate-200 text-sm">
                  <thead className="bg-slate-50">
                    <tr>
                      <th className="px-3 py-2 text-left">Row</th>
                      <th className="px-3 py-2 text-left">Username</th>
                      <th className="px-3 py-2 text-left">Password</th>
                      <th className="px-3 py-2 text-left">License key</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100 bg-white">
                    {result.created.map((c) => (
                      <tr key={`${c.row}-${c.username}`}>
                        <td className="px-3 py-2 text-slate-600">{c.row}</td>
                        <td className="px-3 py-2 text-slate-600">{c.username}</td>
                        <td className="px-3 py-2 font-mono text-slate-600">{c.password}</td>
                        <td className="px-3 py-2 font-mono text-slate-600">{c.license_key}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </Card>
      )}
    </div>
  )
}
