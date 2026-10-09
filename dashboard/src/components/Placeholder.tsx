export function Placeholder({ title }: { title: string }) {
  return (
    <div>
      <h1 className="text-2xl font-bold text-slate-800">{title}</h1>
      <p className="mt-2 text-sm text-slate-500">Coming in a later change-set.</p>
    </div>
  )
}
