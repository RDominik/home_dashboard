import React, { useEffect, useMemo, useState } from 'react'
import Options from './EnergyFlowAnimated.options.jsx'

const fmtW = v => new Intl.NumberFormat('de-DE').format(Math.round(v || 0)) + ' W'

// Animated Energy Flow inspired by Home Assistant cards
export default function EnergyFlowAnimated(){
  const [d, setD] = useState(null)
  const [err, setErr] = useState(null)
  const [opts, setOpts] = useState(() => {
    try { return JSON.parse(localStorage.getItem('efa.opts')||'') || {} } catch { return {} }
  })
  const refreshMs = Number(opts.refreshMs ?? 3000)
  const maxPower = Number(opts.maxPower ?? 5000)
  const speed = Number(opts.speed ?? 1.1) // seconds per cycle
  const showLegend = opts.showLegend !== false

  useEffect(() => { localStorage.setItem('efa.opts', JSON.stringify(opts)) }, [opts])

  useEffect(() => {
    const base = window.location.origin.replace(':8080', ':8081')
    let abort = false
    const tick = async () => {
      try {
        const r = await fetch(base + '/api/inverter/summary', { cache: 'no-store' })
        if (!r.ok) throw new Error('HTTP ' + r.status)
        const j = await r.json()
        if (!abort) { setD(j); setErr(null) }
      } catch (e) {
        if (!abort) setErr('Keine Live-Daten (API erreichbar?)')
      }
    }
    tick()
    const t = setInterval(tick, Math.max(1000, refreshMs))
    return () => { abort = true; clearInterval(t) }
  }, [refreshMs])

  const flows = useMemo(() => {
    const ppv = Number(d?.ppv ?? 0)
    const house = Number(d?.house_consumption ?? 0)
    const diff = ppv - house
    const grid = diff
    const batterySoc = Number(d?.battery_soc ?? 0)
    return { ppv, house, grid, batterySoc }
  }, [d])

  const cap = v => Math.min(1, Math.max(0.12, v/Math.max(500, maxPower)))

  return (
    <div>
      <h1>Energiefluss (animiert)</h1>
      <p style={{color:'#666', marginTop:-6}}>Pfeile laufen in Flussrichtung. Live‑Update alle {Math.round(refreshMs/1000)}s.</p>
      <Options opts={opts} setOpts={setOpts} />
      {err && <div style={{background:'#fff3cd', border:'1px solid #ffeeba', color:'#856404', padding:10, borderRadius:6, margin:'8px 0'}}>{err}</div>}
      <div style={{display:'grid', gridTemplateColumns:'1fr 320px', gap:16}}>
        <Diagram flows={flows} cap={cap} speed={speed} />
        <Panel flows={flows} showLegend={showLegend} />
      </div>
    </div>
  )
}

function Diagram({ flows, cap, speed }){
  const arrow = (x1,y1,x2,y2,v,color, dir='forward') => {
    const w = 8 + 18*cap(Math.abs(v))
    const cls = dir === 'reverse' ? 'flow flow-rev' : 'flow'
    return (
      <g>
        <defs>
          <marker id={`arrow-${color}`} markerWidth="10" markerHeight="10" refX="9" refY="3" orient="auto" markerUnits="strokeWidth">
            <path d="M0,0 L0,6 L9,3 z" fill={color} />
          </marker>
        </defs>
        <line x1={x1} y1={y1} x2={x2} y2={y2}
          stroke={color} strokeWidth={w} strokeLinecap="round"
          markerEnd={`url(#arrow-${color})`} className={cls} />
      </g>
    )
  }

  const gridFlow = flows.grid
  const gridDir = gridFlow < 0 ? 'reverse' : 'forward'

  return (
    <svg viewBox="0 0 680 380" style={{width:'100%', height:380, background:'#f8fafc', border:'1px solid #e5e7eb', borderRadius:10}}>
      <style>{`
        .box { fill: #fff; stroke: #e5e7eb; }
        .label { font: 12px 'Poppins', system-ui, sans-serif; fill: #374151; }
        .value { font: 14px 'Poppins', system-ui, sans-serif; font-weight:600; fill:#111827; }
        .flow { stroke-dasharray: 12 14; animation: dash ${speed}s linear infinite; }
        .flow-rev { animation-direction: reverse; }
        @keyframes dash { to { stroke-dashoffset: -52; } }
      `}</style>
      <rect x="40" y="40" width="150" height="80" rx="10" className="box" />
      <text x="115" y="68" textAnchor="middle" className="label">PV</text>
      <text x="115" y="95" textAnchor="middle" className="value">{fmtW(flows.ppv)}</text>

      <rect x="265" y="40" width="150" height="80" rx="10" className="box" />
      <text x="340" y="68" textAnchor="middle" className="label">Wechselrichter</text>

      <rect x="510" y="40" width="150" height="80" rx="10" className="box" />
      <text x="585" y="68" textAnchor="middle" className="label">Haus</text>
      <text x="585" y="95" textAnchor="middle" className="value">{fmtW(flows.house)}</text>

      <rect x="265" y="190" width="150" height="80" rx="10" className="box" />
      <text x="340" y="218" textAnchor="middle" className="label">Netz</text>
      <text x="340" y="245" textAnchor="middle" className="value">{flows.grid<0?`Import ${fmtW(Math.abs(flows.grid))}`:flows.grid>0?`Export ${fmtW(flows.grid)}`:'—'}</text>

      <rect x="40" y="190" width="150" height="80" rx="10" className="box" />
      <text x="115" y="218" textAnchor="middle" className="label">Batterie</text>
      <text x="115" y="245" textAnchor="middle" className="value">{(flows.batterySoc??0).toFixed(0)}% SoC</text>

      {arrow(190,80, 265,80, flows.ppv, '#f59e0b', 'forward')}
      {arrow(415,80, 510,80, Math.max(1, Math.min(flows.ppv, flows.house)), '#10b981', 'forward')}
      {Math.abs(flows.grid) > 0.5 && arrow(340,190, 340,120, Math.abs(flows.grid), flows.grid<0?'#3b82f6':'#6366f1', flows.grid<0?'reverse':'forward')}
    </svg>
  )
}

function Panel({ flows, showLegend=true }){
  const rows = [
    ['PV', fmtW(flows.ppv)],
    ['Haus', fmtW(flows.house)],
    ['Netz', flows.grid<0?`Import ${fmtW(Math.abs(flows.grid))}`:flows.grid>0?`Export ${fmtW(flows.grid)}`:'—'],
    ['Batterie‑SoC', (flows.batterySoc??0).toFixed(0) + ' %'],
  ]
  const dot = c => <span style={{display:'inline-block', width:10, height:10, borderRadius:10, background:c, marginRight:8}}/>
  return (
    <div style={{border:'1px solid #e5e7eb', borderRadius:10, padding:12, background:'#fff'}}>
      <h3 style={{marginTop:0}}>Werte</h3>
      <ul style={{listStyle:'none', padding:0, margin:0, display:'grid', gap:8}}>
        {rows.map(([k,v]) => (
          <li key={k} style={{display:'flex', justifyContent:'space-between'}}>
            <span style={{color:'#6b7280'}}>{k}</span>
            <span style={{fontWeight:600}}>{v}</span>
          </li>
        ))}
      </ul>
      {showLegend && (
        <>
          <h4 style={{marginTop:16, color:'#374151'}}>Legende</h4>
          <div style={{display:'grid', gap:6, fontSize:13, color:'#4b5563'}}>
            <div>{dot('#f59e0b')} PV → Wechselrichter</div>
            <div>{dot('#10b981')} Wechselrichter → Haus</div>
            <div>{dot('#3b82f6')} Netz → Wechselrichter (Import)</div>
            <div>{dot('#6366f1')} Wechselrichter → Netz (Export)</div>
          </div>
        </>
      )}
    </div>
  )
}
