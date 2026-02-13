import React from 'react'

export default function DashboardHome() {
  return (
    <div>
      <h1>Übersicht</h1>
      <p>Willkommen zur ETA Weboberfläche. Wähle links ein Dashboard.</p>
      <ul>
        <li>Wallbox-Status (go-eCharger)</li>
        <li>Wechselrichter / PV</li>
        <li>Haushaltsverbrauch</li>
      </ul>
    </div>
  )
}
