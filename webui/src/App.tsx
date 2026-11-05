import { useEffect, useState } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import packageJson from '../package.json'
import GlobalStatusBar from './components/GlobalStatusBar'
import './App.css'

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
      <aside className="app-sidebar">
        <h2 className="app-sidebar-title">Dashboards
          
        </h2>
        <nav className="app-sidebar-nav">
          <NavLink to="/" className="app-sidebar-link" end>Übersicht</NavLink>
          <NavLink to="/energy" className="app-sidebar-link">PV Energiefluss</NavLink>
          <NavLink to="/enyaq" className="app-sidebar-link">Škoda Enyaq</NavLink>
          <NavLink to="/goE" className="app-sidebar-link">goE</NavLink>
          <NavLink to="/huehnerklappe" className="app-sidebar-link">Motor</NavLink>
          <NavLink to="/weather" className="app-sidebar-link">Wetterstation</NavLink>
          <NavLink to="/abfallkalender" className="app-sidebar-link">Abfallkalender</NavLink>
          <NavLink to="/heating" className="app-sidebar-link">ETA Heizung</NavLink>
          <NavLink to="/grafana" className="app-sidebar-link">Grafana</NavLink>
          <NavLink to="/update" className="app-sidebar-link">Update</NavLink>
        </nav>
        <div className="app-sidebar-footer">
          <div style={{ fontWeight: 700, marginBottom: 4 }}>Uhrzeit</div>
          <div>{currentTime}</div>
          <div>v{packageJson.version} (Preview)</div>
        </div>
      </aside>
      <div className="app-content">
        <main className="app-main">
          <Outlet />
        </main>
        <GlobalStatusBar />
      </div>
    </div>
  )
}
