import PageHeader from '../components/PageHeader'

export default function DashboardHome() {
  return (
    <div>
      <PageHeader eyebrow="SYSTEMÜBERSICHT" title="Übersicht" subtitle="System-Dashboard" />
      <p>Willkommen zur ETA Weboberfläche. Wähle links ein Dashboard.</p>
      <ul>
        <li>Wallbox-Status (go-eCharger)</li>
        <li>Wechselrichter / PV</li>
        <li>Haushaltsverbrauch</li>
      </ul>
    </div>
  )
}