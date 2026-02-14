import React from 'react'
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer, Legend, CartesianGrid, LineChart, Line } from 'recharts'

export function AreaChartCard({ title, data, lines, height=220, valueFormatter }){
  const fmt = valueFormatter || ((v)=> new Intl.NumberFormat('de-DE').format(Math.round(v)))
  return (
    <div style={{border:'1px solid #eee', borderRadius:8, padding:12}}>
      <h3 style={{marginTop:0}}>{title}</h3>
      <div style={{width:'100%', height}}>
        <ResponsiveContainer>
          <AreaChart data={data} margin={{ top: 10, right: 12, left: 0, bottom: 0 }}>
            <defs>
              {lines.map(l => (
                <linearGradient key={l.dataKey} id={`g-${l.dataKey}`} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor={l.color} stopOpacity={0.4}/>
                  <stop offset="95%" stopColor={l.color} stopOpacity={0}/>
                </linearGradient>
              ))}
            </defs>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="t" hide/>
            <YAxis tickFormatter={v=>fmt(v)} width={60} />
            <Tooltip formatter={(v)=>fmt(v)} labelFormatter={(l)=>new Date(l).toLocaleString('de-DE')} />
            <Legend />
            {lines.map(l => (
              <Area key={l.dataKey} type="monotone" dataKey={l.dataKey} stroke={l.color} fillOpacity={1} fill={`url(#g-${l.dataKey})`} name={l.name} />
            ))}
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}

export function LineChartCard({ title, data, lines, height=220, valueFormatter }){
  const fmt = valueFormatter || ((v)=> new Intl.NumberFormat('de-DE').format(Math.round(v)))
  return (
    <div style={{border:'1px solid #eee', borderRadius:8, padding:12}}>
      <h3 style={{marginTop:0}}>{title}</h3>
      <div style={{width:'100%', height}}>
        <ResponsiveContainer>
          <LineChart data={data} margin={{ top: 10, right: 12, left: 0, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="t" hide/>
            <YAxis tickFormatter={v=>fmt(v)} width={60} />
            <Tooltip formatter={(v)=>fmt(v)} labelFormatter={(l)=>new Date(l).toLocaleString('de-DE')} />
            <Legend />
            {lines.map(l => (
              <Line key={l.dataKey} type="monotone" dataKey={l.dataKey} stroke={l.color} dot={false} name={l.name} />
            ))}
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}
