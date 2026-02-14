import React from 'react'
import { NavLink, Outlet } from 'react-router-dom'

const linkStyle = ({ isActive }) => ({
  display: 'block',
  padding: '10px 12px',
  borderRadius: 6,
  color: isActive ? '#111' : '#333',
  background: isActive ? '#e6f4ff' : 'transparent',
  textDecoration: 'none',
  fontWeight: 500,
})

export default function App() {
  return (
    <div style={{ display: 'flex', height: '100vh' }}>
      <aside style={{ width: 260, borderRight: '1px solid #e5e7eb', padding: 16, background:'#f9fafb' }}>
        <h2 style={{ marginTop: 0, letterSpacing: '0.2px' }}>ETA</h2>
        <nav style={{ display: 'grid', gap: 6 }}>
          <NavLink to="/" style={linkStyle} end>Übersicht</NavLink>
          <NavLink to="/wallbox" style={linkStyle}>Wallbox</NavLink>
          <NavLink to="/inverter" style={linkStyle}>Wechselrichter</NavLink>
          <NavLink to="/energy" style={linkStyle}>PV Energiefluss</NavLink>
          <NavLink to="/heating" style={linkStyle}>ETA Heizung</NavLink>
          <NavLink to="/grafana" style={linkStyle}>Grafana</NavLink>
        </nav>
        <div style={{ marginTop: 24, fontSize: 12, color: '#666' }}>
          <div>v0.1 (Preview)</div>
        </div>
      </aside>
      <main style={{ flex: 1, padding: 20, overflow: 'auto' }}>
        <Outlet />
      </main>
    </div>
  )
}
