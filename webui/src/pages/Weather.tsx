import type { CSSProperties } from 'react'
import { useEffect, useState } from 'react'

const API = '/api/weather'

type WeatherSettings = {
  stationId: string
  apiKey: string
  units: 'metric' | 'imperial'
  intervalSeconds: number
  maxRequestsPerMinute: number
}

type Observation = {
  stationId: string
  stationName: string
  observedAt: string
  temperature: number
  feelsLike: number
  humidity: number
  windSpeed: number
  windGust: number
  windDirection: string
  pressure: number
  dewPoint: number
  precipRate: number
  precipTotal: number
  uv: number
  solarRadiation: number
  latitude: number
  longitude: number
  elevation: number
  temperatureUnit: string
  windUnit: string
  pressureUnit: string
  precipUnit: string
  updatedAt: string
}

type WeatherResponse = {
  settings: WeatherSettings
  observation?: Observation
  lastFetchAt?: string
  error?: string
  configured: boolean
}

const initialSettings: WeatherSettings = {
  stationId: 'IRIEDE66',
  apiKey: '',
  units: 'metric',
  intervalSeconds: 300,
  maxRequestsPerMinute: 1,
}

function formatNumber(value: number | undefined, digits = 1) {
  return typeof value === 'number' && Number.isFinite(value) ? value.toFixed(digits) : '—'
}

function formatTime(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('de-DE')
}

export default function Weather() {
  const [data, setData] = useState<WeatherResponse | null>(null)
  const [settings, setSettings] = useState<WeatherSettings>(initialSettings)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [message, setMessage] = useState('')

  const loadStatus = async () => {
    try {
      const response = await fetch(`${API}/status`)
      if (!response.ok) return
      const next: WeatherResponse = await response.json()
      setData(next)
      setSettings(next.settings)
    } catch {
      setMessage('Wetterdienst ist nicht erreichbar.')
    }
  }

  useEffect(() => {
    loadStatus()
    const timer = window.setInterval(loadStatus, 60_000)
    return () => window.clearInterval(timer)
  }, [])

  const saveSettings = async () => {
    setSaving(true)
    setMessage('')
    try {
      const response = await fetch(`${API}/settings`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(settings),
      })
      const result = await response.json()
      if (!response.ok) throw new Error(result.error ?? 'Speichern fehlgeschlagen')
      setSettingsOpen(false)
      setMessage('Einstellungen gespeichert. Wetterdaten werden aktualisiert.')
      await loadStatus()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Speichern fehlgeschlagen')
    } finally {
      setSaving(false)
    }
  }

  const refresh = async () => {
    setRefreshing(true)
    try {
      await fetch(`${API}/refresh`, { method: 'POST' })
      window.setTimeout(loadStatus, 700)
    } finally {
      setRefreshing(false)
    }
  }

  const observation = data?.observation
  const accent = '#0f766e'
  const panel: CSSProperties = {
    background: 'rgba(255,255,255,0.92)',
    border: '1px solid #dbe4e5',
    borderRadius: 14,
    boxShadow: '0 10px 30px rgba(15, 59, 64, 0.08)',
  }

  return (
    <div style={{ maxWidth: 1180, margin: '0 auto', color: '#20343a' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 16, alignItems: 'flex-start', flexWrap: 'wrap', marginBottom: 22 }}>
        <div>
          <div style={{ color: accent, fontSize: 12, fontWeight: 800, letterSpacing: 1.5, textTransform: 'uppercase' }}>Personal Weather Station</div>
          <h1 style={{ margin: '6px 0 4px', fontSize: 34 }}>Wetterstation {settings.stationId}</h1>
          <p style={{ margin: 0, color: '#6a7b80' }}>{observation?.stationName || 'Weather Underground Station Dashboard'}</p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button type="button" onClick={refresh} disabled={refreshing} style={{ ...buttonStyle('#e5f3f0', accent), opacity: refreshing ? 0.6 : 1 }}>↻ Aktualisieren</button>
          <button type="button" onClick={() => setSettingsOpen(true)} style={buttonStyle(accent, '#fff')}>⚙ Einstellungen</button>
        </div>
      </div>

      {message && <div style={{ ...panel, padding: '12px 16px', marginBottom: 16, color: accent }}>{message}</div>}
      {data?.error && <div style={{ ...panel, padding: '14px 16px', marginBottom: 16, borderColor: '#f0caca', color: '#a33a3a' }}>{data.error}</div>}

      {!data?.configured ? (
        <div style={{ ...panel, padding: 34, textAlign: 'center' }}>
          <div style={{ fontSize: 42, marginBottom: 10 }}>☁</div>
          <h2 style={{ margin: '0 0 8px' }}>Wetterstation einrichten</h2>
          <p style={{ color: '#6a7b80', margin: '0 0 18px' }}>Station-ID und Weather-Underground-API-Key werden einmalig in den Einstellungen hinterlegt.</p>
          <button type="button" onClick={() => setSettingsOpen(true)} style={buttonStyle(accent, '#fff')}>Einstellungen öffnen</button>
        </div>
      ) : observation ? (
        <>
          <section style={{ ...panel, padding: 22, background: 'linear-gradient(135deg, #e9f6f4 0%, #ffffff 62%)', display: 'grid', gridTemplateColumns: 'minmax(240px, 1.2fr) repeat(3, minmax(130px, 1fr))', gap: 18, alignItems: 'center' }}>
            <div>
              <div style={{ color: '#6a7b80', fontSize: 13 }}>Aktuelle Bedingungen</div>
              <div style={{ fontSize: 64, lineHeight: 1, fontWeight: 800, color: '#123f45', margin: '8px 0' }}>{formatNumber(observation.temperature)}{observation.temperatureUnit}</div>
              <div style={{ color: '#567177' }}>Gefühlt {formatNumber(observation.feelsLike)}{observation.temperatureUnit} · aktualisiert {formatTime(observation.observedAt)}</div>
            </div>
            <Metric label="Luftfeuchte" value={`${formatNumber(observation.humidity, 0)} %`} />
            <Metric label="Taupunkt" value={`${formatNumber(observation.dewPoint)}${observation.temperatureUnit}`} />
            <Metric label="UV-Index" value={formatNumber(observation.uv, 0)} />
          </section>

          <section style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(210px, 1fr))', gap: 14, marginTop: 16 }}>
            <MetricCard title="Wind & Böen" value={`${formatNumber(observation.windSpeed)} ${observation.windUnit}`} detail={`${observation.windDirection || '—'} · Böen ${formatNumber(observation.windGust)} ${observation.windUnit}`} />
            <MetricCard title="Luftdruck" value={`${formatNumber(observation.pressure)} ${observation.pressureUnit}`} detail={`Station ${observation.stationId}`} />
            <MetricCard title="Niederschlag" value={`${formatNumber(observation.precipRate)} ${observation.precipUnit}/h`} detail={`Summe ${formatNumber(observation.precipTotal)} ${observation.precipUnit}`} />
            <MetricCard title="Solarstrahlung" value={`${formatNumber(observation.solarRadiation, 0)} W/m²`} detail={`Höhe ${formatNumber(observation.elevation, 0)} m`} />
          </section>

          <section style={{ ...panel, marginTop: 16, padding: 18, display: 'flex', justifyContent: 'space-between', gap: 14, flexWrap: 'wrap', color: '#6a7b80', fontSize: 13 }}>
            <span>Koordinaten {formatNumber(observation.latitude, 3)}° N · {formatNumber(observation.longitude, 3)}° E</span>
            <span>Letzter Abruf: {formatTime(data.lastFetchAt)}</span>
          </section>
        </>
      ) : (
        <div style={{ ...panel, padding: 32, textAlign: 'center', color: '#6a7b80' }}>Wetterdaten werden geladen …</div>
      )}

      {settingsOpen && <SettingsModal settings={settings} setSettings={setSettings} saving={saving} onClose={() => setSettingsOpen(false)} onSave={saveSettings} />}
    </div>
  )
}

function buttonStyle(background: string, color: string): CSSProperties {
  return { border: '1px solid #b7d7d2', borderRadius: 9, background, color, padding: '10px 14px', fontWeight: 800, cursor: 'pointer' }
}

function Metric({ label, value }: { label: string; value: string }) {
  return <div><div style={{ color: '#6a7b80', fontSize: 12, marginBottom: 6 }}>{label}</div><div style={{ fontSize: 25, fontWeight: 800, color: '#1d4e55' }}>{value}</div></div>
}

function MetricCard({ title, value, detail }: { title: string; value: string; detail: string }) {
  return <div style={{ background: '#fff', border: '1px solid #dbe4e5', borderRadius: 14, padding: 18, boxShadow: '0 7px 20px rgba(15, 59, 64, 0.06)' }}><div style={{ color: '#6a7b80', fontSize: 13, fontWeight: 700 }}>{title}</div><div style={{ color: '#203f45', fontSize: 25, fontWeight: 800, margin: '10px 0 6px' }}>{value}</div><div style={{ color: '#71858a', fontSize: 13 }}>{detail}</div></div>
}

function SettingsModal({ settings, setSettings, saving, onClose, onSave }: { settings: WeatherSettings; setSettings: (value: WeatherSettings) => void; saving: boolean; onClose: () => void; onSave: () => void }) {
  return <div role="dialog" aria-modal="true" style={{ position: 'fixed', inset: 0, zIndex: 20, background: 'rgba(19, 39, 43, 0.45)', display: 'grid', placeItems: 'center', padding: 20 }}>
    <div style={{ width: 'min(100%, 520px)', background: '#fff', borderRadius: 16, padding: 24, boxShadow: '0 20px 60px rgba(0,0,0,0.22)' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}><h2 style={{ margin: 0 }}>Wetter-Einstellungen</h2><button type="button" onClick={onClose} aria-label="Einstellungen schließen" style={{ border: 0, background: 'transparent', fontSize: 24, cursor: 'pointer' }}>×</button></div>
      <p style={{ color: '#6a7b80', fontSize: 13 }}>Die Werte werden serverseitig in bbolt gespeichert und nach dem Speichern sofort verwendet.</p>
      <label style={labelStyle}>Station-ID<input value={settings.stationId} onChange={event => setSettings({ ...settings, stationId: event.target.value.toUpperCase() })} placeholder="IRIEDE66" style={inputStyle} /></label>
      <label style={labelStyle}>Weather-Underground-API-Key<input type="password" value={settings.apiKey} onChange={event => setSettings({ ...settings, apiKey: event.target.value })} placeholder="API-Key" style={inputStyle} /></label>
      <label style={labelStyle}>Einheiten<select value={settings.units} onChange={event => setSettings({ ...settings, units: event.target.value as WeatherSettings['units'] })} style={inputStyle}><option value="metric">Metrisch (°C, km/h, hPa)</option><option value="imperial">Imperial (°F, mph, inHg)</option></select></label>
      <label style={labelStyle}>Abrufintervall in Sekunden<input type="number" min={30} max={3600} value={settings.intervalSeconds} onChange={event => setSettings({ ...settings, intervalSeconds: Math.max(30, Math.min(3600, Number(event.target.value) || 30)) })} style={inputStyle} /></label>
      <label style={labelStyle}>Maximale Abfragen pro Minute<input type="number" min={1} max={5} value={settings.maxRequestsPerMinute} onChange={event => setSettings({ ...settings, maxRequestsPerMinute: Math.max(1, Math.min(5, Number(event.target.value) || 1)) })} style={inputStyle} /><span style={{ color: '#6a7b80', fontSize: 12, fontWeight: 500 }}>Weather Underground erlaubt für diesen PWS-Endpunkt bis zu 5 Abfragen pro Minute pro API-Key. Der Server erzwingt diesen Wert.</span></label>
      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10, marginTop: 22 }}><button type="button" onClick={onClose} style={buttonStyle('#f3f7f7', '#31565c')}>Abbrechen</button><button type="button" onClick={onSave} disabled={saving} style={buttonStyle('#0f766e', '#fff')}>{saving ? 'Speichert …' : 'Speichern'}</button></div>
    </div>
  </div>
}

const labelStyle: CSSProperties = { display: 'grid', gap: 6, marginTop: 16, color: '#36565c', fontSize: 13, fontWeight: 800 }
const inputStyle: CSSProperties = { width: '100%', boxSizing: 'border-box', border: '1px solid #c5d8d8', borderRadius: 8, padding: '10px 11px', fontSize: 14, color: '#20343a', background: '#fbfdfd' }
