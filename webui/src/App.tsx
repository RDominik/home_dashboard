import type { CSSProperties } from 'react'
import { useEffect, useState } from 'react'
import { NavLink, Outlet } from 'react-router-dom'

const linkStyle = ({ isActive }: { isActive: boolean }): CSSProperties => ({
  display: 'block',
  padding: '10px 12px',
  borderRadius: 6,
  color: isActive ? '#111' : '#333',
  background: isActive ? '#e6f4ff' : 'transparent',
  textDecoration: 'none',
  fontWeight: 500,
})

export default function App() {
  const [now, setNow] = useState(new Date())

  useEffect(() => {
    const timer = setInterval(() => setNow(new Date()), 1000)
    return () => clearInterval(timer)
  }, [])

  const currentTime = new Intl.DateTimeFormat('de-DE', {
    dateStyle: 'medium',
    timeStyle: 'medium',
  }).format(now)

  return (
    <div style={{ position: 'fixed', inset: 0, display: 'flex', overflow: 'hidden', background: '#cfcbcbea' }}>
      <aside style={{ width: 260, flexShrink: 0, overflow: 'hidden', borderRight: '1px solid #020202', padding: 16, background:'#cac6c6a9', display: 'flex', flexDirection: 'column' }}>
        <h2 style={{ marginTop: 0, letterSpacing: '0.2px' }}>Dashboards
          
        </h2>
        <nav style={{ display: 'grid', gap: 6 }}>
          <NavLink to="/" style={linkStyle} end>Übersicht</NavLink>
          <NavLink to="/energy" style={linkStyle}>PV Energiefluss</NavLink>
          <NavLink to="/goE" style={linkStyle}>goE</NavLink>
          <NavLink to="/huehnerklappe" style={linkStyle}>Motor</NavLink>
          <NavLink to="/heating" style={linkStyle}>ETA Heizung</NavLink>
          <NavLink to="/grafana" style={linkStyle}>Grafana</NavLink>
          <NavLink to="/update" style={linkStyle}>Update</NavLink>
        </nav>
        <div style={{ marginTop: 'auto', paddingTop: 24, fontSize: 12, color: '#050505' }}>
          <div style={{ fontWeight: 700, marginBottom: 4 }}>Uhrzeit</div>
          <div>{currentTime}</div>
          <div>v0.1 (Preview)</div>
        </div>
      </aside>
      <main style={{ flex: 1, minWidth: 0, minHeight: 0, padding: 20, overflowY: 'auto', overflowX: 'hidden', overflowAnchor: 'none' }}>
        <Outlet />
      </main>
    </div>
  )
}
