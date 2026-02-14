import React from 'react'

export default function Options({ opts, setOpts }){
  const field = (label, input) => (
    <label style={{display:'grid', gap:6}}>
      <span style={{fontSize:12, color:'#6b7280'}}>{label}</span>
      {input}
    </label>
  )
  return (
    <div style={{display:'grid', gridTemplateColumns:'repeat(auto-fill, minmax(180px, 1fr))', gap:12, margin:'8px 0 12px'}}>
      {field('Refresh‑Intervall (Sek.)', (
        <input type="number" min={1} max={30} value={Math.round((opts.refreshMs??3000)/1000)} onChange={e=>setOpts(o=>({...o, refreshMs: Math.max(1000, Number(e.target.value||3)*1000)}))} style={{padding:'8px 10px', border:'1px solid #e5e7eb', borderRadius:8}} />
      ))}
      {field('Max‑Leistung für Pfeildicke (W)', (
        <input type="number" min={500} step={100} value={opts.maxPower??5000} onChange={e=>setOpts(o=>({...o, maxPower: Math.max(500, Number(e.target.value||5000))}))} style={{padding:'8px 10px', border:'1px solid #e5e7eb', borderRadius:8}} />
      ))}
      {field('Animations‑Geschwindigkeit (s/Zyklus)', (
        <input type="number" min={0.4} step={0.1} value={opts.speed??1.1} onChange={e=>setOpts(o=>({...o, speed: Math.max(0.3, Number(e.target.value||1.1))}))} style={{padding:'8px 10px', border:'1px solid #e5e7eb', borderRadius:8}} />
      ))}
      <label style={{display:'flex', alignItems:'center', gap:8, paddingTop:20}}>
        <input type="checkbox" checked={opts.showLegend !== false} onChange={e=>setOpts(o=>({...o, showLegend: e.target.checked}))} />
        <span style={{color:'#374151'}}>Legende anzeigen</span>
      </label>
    </div>
  )
}
