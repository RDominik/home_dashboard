import React, { useEffect, useState } from 'react'

export default function Heating() {
  const [d, setD] = useState(null)
  useEffect(() => {
    const base = window.location.origin.replace(':8080', ':8081')
    const tick = () => fetch(base + '/api/heating/summary').then(r=>r.json()).then(setD).catch(()=>{})
    tick()
    const t = setInterval(tick, 5000)
    return () => clearInterval(t)
  }, [])

  return (
    <div>
      <h1>ETA Hackschnitzel‑Heizung</h1>
      <p style={{color:'#666', marginTop:-6}}>Live‑Werte alle 5s</p>
      <Cards d={d} />
      <Hints />
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

function Cards({ d }){
  const grid = {
    display:'grid',
    gridTemplateColumns:'repeat(auto-fill, minmax(180px, 1fr))',
    gap:12,
    marginTop:12
  }
  return (
    <div style={grid}>
      <Card title="Kesseltemperatur" value={fmt(d?.boiler_temp)} unit="°C" />
      <Card title="Puffer (oben)" value={fmt(d?.buffer_top)} unit="°C" />
      <Card title="Puffer (unten)" value={fmt(d?.buffer_bottom)} unit="°C" />
      <Card title="Rücklauf" value={fmt(d?.return_temp)} unit="°C" />
      <Card title="Außentemperatur" value={fmt(d?.outside_temp)} unit="°C" />
      <Card title="Förderrate" value={fmt(d?.feed_rate)} unit="%" />
      <Card title="Brenner" value={d?.burner_status === 'on' ? 'An' : 'Aus'} />
    </div>
  )
}

function Hints(){
  return (
    <div style={{marginTop:16, color:'#666', fontSize:12}}>
      <div>Hinweis: Werte sind aktuell Mock‑Daten. Backend liefert /api/heating/summary; reale Sensorwerte/Topics können wir anbinden (MQTT/Modbus/REST).</div>
    </div>
  )
}

function fmt(v){
  if (v == null) return null
  return new Intl.NumberFormat('de-DE', { maximumFractionDigits: 1 }).format(v)
}
