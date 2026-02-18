import React, { useEffect, useState } from 'react'
import { NavLink, Outlet } from 'react-router-dom'

const linkStyle = ({ isActive }) => ({
  display: 'flex',
  alignItems: 'center',
  gap: 10,
  padding: '10px 12px',
  borderRadius: 6,
  color: isActive ? '#111' : '#333',
  background: isActive ? '#e6f4ff' : 'transparent',
  textDecoration: 'none',
  fontWeight: 500,
  whiteSpace: 'nowrap',
})

function ItemIcon({ children }){
  return (
    <span aria-hidden style={{display:'inline-flex', width:22, justifyContent:'center'}}>{children}</span>
  )
}

const btnStyle = {
  border:'1px solid #ddd',
  background:'#fff',
  borderRadius:6,
  padding:'4px 8px',
  cursor:'pointer'
}

export default function App() {
  const [collapsed, setCollapsed] = useState(false)
  useEffect(() => { try { setCollapsed(localStorage.getItem('sidebar.collapsed') === '1') } catch {} }, [])
  useEffect(() => { try { localStorage.setItem('sidebar.collapsed', collapsed ? '1' : '0') } catch {} }, [collapsed])

  const asideStyle = collapsed
    ? { width: 64, borderRight: '1px solid #020202', padding: 12, background:'#cac6c6a9', display:'flex', flexDirection:'column', alignItems:'center' }
    : { width: 260, borderRight: '1px solid #020202', padding: 16, background:'#cac6c6a9' }

  const label = txt => collapsed ? null : <span>{txt}</span>

  return (
    <div style={{ display: 'flex', height: '100vh', background: '#cfcbcbea' }}>
      <aside style={asideStyle}>
        <div style={{ display:'flex', alignItems:'center', justifyContent: collapsed ? 'center' : 'space-between', width:'100%' }}>
          <h2 style={{ margin: 0, letterSpacing: '0.2px' }}>{collapsed ? '☰' : 'Dashboards'}</h2>
          {!collapsed && (
            <button onClick={() => setCollapsed(true)} title="Seitenleiste einklappen" style={btnStyle}>«</button>
          )}
          {collapsed && (
            <button onClick={() => setCollapsed(false)} title="Seitenleiste ausklappen" style={{...btnStyle, padding:'6px 8px'}}>»</button>
          )}
        </div>
        <nav style={{ display: 'grid', gap: 6, marginTop: 12 }}>
          <NavLink to="/" style={linkStyle} end title="Übersicht">
            <ItemIcon>🏠</ItemIcon>{label('Übersicht')}
          </NavLink>
          <NavLink to="/wallbox" style={linkStyle} title="Wallbox">
            <ItemIcon>🔌</ItemIcon>{label('Wallbox')}
          </NavLink>
          <NavLink to="/inverter" style={linkStyle} title="Wechselrichter">
            <ItemIcon>⚙️</ItemIcon>{label('Wechselrichter')}
          </NavLink>
          <NavLink to="/energy" style={linkStyle} title="PV Energiefluss">
            <ItemIcon>☀️</ItemIcon>{label('PV Energiefluss')}
          </NavLink>
          <NavLink to="/energy-animated" style={linkStyle} title="Energiefluss (animiert)">
            <ItemIcon>➡️</ItemIcon>{label('Energiefluss (animiert)')}
          </NavLink>
          <NavLink to="/heating" style={linkStyle} title="ETA Heizung">
            <ItemIcon>🔥</ItemIcon>{label('ETA Heizung')}
          </NavLink>
          <NavLink to="/grafana" style={linkStyle} title="Grafana">
            <ItemIcon>📊</ItemIcon>{label('Grafana')}
          </NavLink>
        </nav>
        {!collapsed && (
          <div style={{ marginTop: 24, fontSize: 12, color: '#050505' }}>
            <div>v0.1 (Preview)</div>
          </div>
        )}
      </aside>
      <main style={{ flex: 1, padding: 20, overflow: 'auto' }}>
        <Outlet />
      </main>
    </div>
  )
}
