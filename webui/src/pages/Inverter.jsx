import React, { useEffect, useMemo, useState } from 'react'
import { AreaChartCard } from '../components/Charts.jsx'

export default function Inverter() {
  const [summary, setSummary] = useState(null)
  const [hist, setHist] = useState([])
  const [err, setErr] = useState(null)

  useEffect(() => {
    const base = window.location.origin.replace(':8080', ':8081')
    let abort = false
    const fetchSummary = async () => {
      try {
        const r = await fetch(base + '/api/inverter/summary', { cache: 'no-store' })
        if (!r.ok) throw new Error('HTTP ' + r.status)
        const j = await r.json(); if (!abort) { setSummary(j); setErr(null) }
      } catch(e){ if (!abort) setErr('Inverter-API nicht erreichbar') }
    }
    const fetchHist = async () => {
      try {
        const r = await fetch(base + '/api/inverter/history?interval=5m', { cache: 'no-store' })
        if (!r.ok) throw new Error('HTTP ' + r.status)
        const j = await r.json(); if (!abort) { setHist(j.series || []); setErr(null) }
      } catch(e){ if (!abort) setErr('Inverter-Historie nicht erreichbar') }
    }
    fetchSummary(); fetchHist()
    const t1 = setInterval(fetchSummary, 4000)
    const t2 = setInterval(fetchHist, 20000)
    return () => { abort = true; clearInterval(t1); clearInterval(t2) }
  }, [])

  const cards = useMemo(() => ([
    ['PV', summary?.ppv, 'W'],
    ['Haus', summary?.house_consumption, 'W'],
    ['Batterie‑SoC', summary?.battery_soc, '%'],
  ]), [summary])

  return (
    <div>
      <h1>Wechselrichter</h1>
      {err && <div style={{background:'#fff3cd', border:'1px solid #ffeeba', color:'#856404', padding:10, borderRadius:6, margin:'8px 0'}}>{err}</div>}
      <Cards items={cards} />
      <div style={{display:'grid', gridTemplateColumns:'1fr', gap:12, marginTop:12}}>
        <AreaChartCard title="PV vs. Haus" data={hist} lines={[
          { dataKey:'ppv', name:'PV', color:'#f59e0b' },
          { dataKey:'house', name:'Haus', color:'#10b981' },
        ]} valueFormatter={v=>new Intl.NumberFormat('de-DE').format(Math.round(v)) + ' W'} />
        <AreaChartCard title="Batterie‑SoC" data={hist} lines={[
          { dataKey:'battery_soc', name:'SoC', color:'#6366f1' },
        ]} valueFormatter={v=>new Intl.NumberFormat('de-DE', { maximumFractionDigits:1 }).format(v) + ' %'} />
      </div>
    </div>
  )
}

function Card({ title, value, unit }){
  return (
    <div style={{border:'1px solid #eee', borderRadius:8, padding:12}}>
      <div style={{color:'#666', fontSize:12}}>{title}</div>
      <div style={{fontSize:22, fontWeight:700}}>{value != null ? fmt(value, unit) : '—'}</div>
    </div>
  )
}

function Cards({ items }){
  const grid = { display:'grid', gridTemplateColumns:'repeat(auto-fill, minmax(160px, 1fr))', gap:12, marginTop:12 }
  return (
    <div style={grid}>
      {items.map(([t,v,u]) => <Card key={t} title={t} value={v} unit={u} />)}
    </div>
  )
}

function fmt(v, unit){
  if (v == null) return null
  if (unit === '%') return new Intl.NumberFormat('de-DE', { maximumFractionDigits:1 }).format(v) + ' %'
  return new Intl.NumberFormat('de-DE').format(Math.round(v)) + ' ' + (unit || '')
}
