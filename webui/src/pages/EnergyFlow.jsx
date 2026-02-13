import React, { useEffect, useMemo, useState } from 'react'

// Simple power-flow diagram (PV → Inverter → House/Grid/Battery) with animated arrows
export default function EnergyFlow() {
  const [d, setD] = useState(null)
  useEffect(() => {
    const base = window.location.origin.replace(':8080', ':8081')
    const tick = () => fetch(base + '/api/inverter/summary').then(r=>r.json()).then(setD).catch(()=>{})
    tick()
    const t = setInterval(tick, 3000)
    return () => clearInterval(t)
  }, [])

  const flows = useMemo(() => {
    const ppv = Number(d?.ppv ?? 0)
    const house = Number(d?.house_consumption ?? 0)
    // naive derivation for grid import/export; battery flow left 0 in MVP
    const diff = ppv - house
    const gridImport = diff < 0 ? Math.abs(diff) : 0
    const gridExport = diff > 0 ? diff : 0
    const batterySoc = Number(d?.battery_soc ?? 0)
    return { ppv, house, gridImport, gridExport, batterySoc }
  }, [d])

  const scale = v => Math.min(1, Math.max(0.1, v / 4000)) // scale 0.1..1 for ~0..4kW
  const fmt = v => new Intl.NumberFormat('de-DE').format(Math.round(v)) + ' W'

  return (
    <div>
      <h1>PV‑Anlage – Energiefluss</h1>
      <p style={{color:'#666', marginTop:-6}}>Live‑Werte alle 3s aktualisiert</p>
      <div style={{display:'grid',gridTemplateColumns:'1fr 1fr',gap:20, alignItems:'start'}}>
        <Diagram flows={flows} scaleFn={scale} />
        <Stats flows={flows} />
      </div>
    </div>
  )
}

function Diagram({ flows, scaleFn }){
  const arrow = (x1,y1,x2,y2,v,color) => {
    const w = 6 + 14*scaleFn(v)
    return (
      <g>
        <defs>
          <marker id={`arrow-${color}`} markerWidth="10" markerHeight="10" refX="9" refY="3" orient="auto" markerUnits="strokeWidth">
            <path d="M0,0 L0,6 L9,3 z" fill={color} />
          </marker>
        </defs>
        <line x1={x1} y1={y1} x2={x2} y2={y2}
          stroke={color} strokeWidth={w} strokeLinecap="round"
          markerEnd={`url(#arrow-${color})`} className="flow" />
      </g>
    )
  }
  return (
    <svg viewBox="0 0 600 360" style={{width:'100%',height:360, background:'#fafafa', border:'1px solid #eee', borderRadius:8}}>
      <style>{`
        .box { fill: #fff; stroke: #ddd; }
        .label { font: 12px system-ui; fill: #333; }
        .value { font: 14px system-ui; font-weight:600; fill:#111; }
        .flow { stroke-dasharray: 8 10; animation: dash 1.2s linear infinite; }
        @keyframes dash { to { stroke-dashoffset: -36; } }
      `}</style>
      {/* Nodes */}
      <rect x="40" y="40" width="140" height="70" rx="8" className="box" />
      <text x="110" y="65" textAnchor="middle" className="label">PV</text>
      <text x="110" y="88" textAnchor="middle" className="value">{fmt(flows.ppv)}</text>

      <rect x="230" y="40" width="140" height="70" rx="8" className="box" />
      <text x="300" y="65" textAnchor="middle" className="label">Wechselrichter</text>

      <rect x="430" y="40" width="140" height="70" rx="8" className="box" />
      <text x="500" y="65" textAnchor="middle" className="label">Haus</text>
      <text x="500" y="88" textAnchor="middle" className="value">{fmt(flows.house)}</text>

      <rect x="230" y="170" width="140" height="70" rx="8" className="box" />
      <text x="300" y="195" textAnchor="middle" className="label">Netz</text>
      <text x="300" y="218" textAnchor="middle" className="value">{flows.gridImport>0?`Import ${fmt(flows.gridImport)}`:flows.gridExport>0?`Export ${fmt(flows.gridExport)}`:'—'}</text>

      <rect x="40" y="170" width="140" height="70" rx="8" className="box" />
      <text x="110" y="195" textAnchor="middle" className="label">Batterie</text>
      <text x="110" y="218" textAnchor="middle" className="value">{flows.batterySoc}% SoC</text>

      {/* Flows */}
      {arrow(180,75, 230,75, flows.ppv, '#f59e0b')}
      {arrow(370,75, 430,75, Math.max(1, Math.min(flows.ppv, flows.house)), '#10b981')}
      {/* Grid import/export arrows */}
      {flows.gridImport>0 && arrow(300,170, 300,110, flows.gridImport, '#3b82f6')}
      {flows.gridExport>0 && arrow(300,110, 300,170, flows.gridExport, '#6366f1')}
    </svg>
  )
}

function Stats({ flows }){
  const rows = [
    ['PV', flows.ppv],
    ['Haus', flows.house],
    ['Netz Import', flows.gridImport],
    ['Netz Export', flows.gridExport],
    ['Batterie‑SoC', flows.batterySoc + ' %'],
  ]
  return (
    <div style={{border:'1px solid #eee', borderRadius:8, padding:12}}>
      <h3 style={{marginTop:0}}>Werte</h3>
      <ul style={{listStyle:'none', padding:0, margin:0, display:'grid', gap:6}}>
        {rows.map(([k,v]) => (
          <li key={k} style={{display:'flex', justifyContent:'space-between'}}>
            <span style={{color:'#666'}}>{k}</span>
            <span style={{fontWeight:600}}>{typeof v === 'number' ? (new Intl.NumberFormat('de-DE').format(Math.round(v)) + ' W') : v}</span>
          </li>
        ))}
      </ul>
    </div>
  )
}

function fmt(v){
  return new Intl.NumberFormat('de-DE').format(Math.round(v)) + ' W'
}
