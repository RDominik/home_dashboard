import React, { useEffect, useState } from 'react'
import { LineChartCard } from '../components/Charts.jsx'

export default function Wallbox() {
  const [status, setStatus] = useState(null)
  const [hist, setHist] = useState([])
  const [err, setErr] = useState(null)

  useEffect(() => {
    const base = window.location.origin.replace(':8080', ':8081')
    let abort = false
    const fetchStatus = async () => {
      try {
        const r = await fetch(base + '/api/wallbox/status', { cache: 'no-store' })
        if (!r.ok) throw new Error('HTTP ' + r.status)
        const j = await r.json(); if (!abort) { setStatus(j); setErr(null) }
      } catch(e){ if (!abort) setErr('Wallbox-API nicht erreichbar') }
    }
    const fetchHist = async () => {
      try {
        const r = await fetch(base + '/api/wallbox/history?interval=5m', { cache: 'no-store' })
        if (!r.ok) throw new Error('HTTP ' + r.status)
        const j = await r.json(); if (!abort) { setHist(j.series || []); setErr(null) }
      } catch(e){ if (!abort) setErr('Wallbox-Historie nicht erreichbar') }
    }
    fetchStatus(); fetchHist()
    const t1 = setInterval(fetchStatus, 5000)
    const t2 = setInterval(fetchHist, 20000)
    return () => { abort = true; clearInterval(t1); clearInterval(t2) }
  }, [])

  return (
    <div>
      <h1>Wallbox</h1>
      {err && <div style={{background:'#fff3cd', border:'1px solid #ffeeba', color:'#856404', padding:10, borderRadius:6, margin:'8px 0'}}>{err}</div>}
      <Cards status={status} />
      <div style={{marginTop:16}}>
        <LineChartCard title="Ladeverlauf (Energy)" data={hist} lines={[{ dataKey:'currentEnergy', name:'Energie', color:'#3b82f6' }]} valueFormatter={v=>new Intl.NumberFormat('de-DE').format(Math.round(v)) + ' Wh'} />
      </div>
    </div>
  )
}

function Card({ title, value, unit }){
  return (
    <div style={{border:'1px solid #eee', borderRadius:8, padding:12}}>
      <div style={{color:'#666', fontSize:12}}>{title}</div>
      <div style={{fontSize:22, fontWeight:700}}>{value != null ? value : '—'}{unit ? ' ' + unit : ''}</div>
    </div>
  )
}

function Cards({ status }){
  const grid = { display:'grid', gridTemplateColumns:'repeat(auto-fill, minmax(160px, 1fr))', gap:12, marginTop:12 }
  return (
    <div style={grid}>
      <Card title="Stromstärke" value={status?.amp} unit="A" />
      <Card title="FRC" value={status?.frc} />
      <Card title="PSM" value={status?.psm} />
      <Card title="CAR" value={status?.car} />
      <Card title="Status" value={status?.modelStatus === 1 ? 'OK' : status?.modelStatus} />
    </div>
  )
}
