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

type WeatherTab = 'today' | 'hourly' | 'tenDay'

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

function createDefaultObservation(stationId: string): Observation {
  return {
    stationId,
    stationName: 'Persönliche Wetterstation',
    observedAt: '',
    temperature: 0,
    feelsLike: 0,
    humidity: 0,
    windSpeed: 0,
    windGust: 0,
    windDirection: '—',
    pressure: 0,
    dewPoint: 0,
    precipRate: 0,
    precipTotal: 0,
    uv: 0,
    solarRadiation: 0,
    latitude: 0,
    longitude: 0,
    elevation: 0,
    temperatureUnit: '°C',
    windUnit: 'km/h',
    pressureUnit: 'hPa',
    precipUnit: 'mm',
    updatedAt: '',
  }
}

export default function Weather() {
  const [data, setData] = useState<WeatherResponse | null>(null)
  const [settings, setSettings] = useState<WeatherSettings>(initialSettings)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [message, setMessage] = useState('')
  const [activeTab, setActiveTab] = useState<WeatherTab>('today')

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
  const displayObservation = observation ?? createDefaultObservation(settings.stationId)
  const accent = '#d7463f'
  const panel: CSSProperties = {
    background: '#fff',
    border: '1px solid #d9e0e5',
    borderRadius: 4,
    boxShadow: '0 3px 12px rgba(27, 49, 67, 0.08)',
  }

  return (
    <div style={{ maxWidth: 1240, margin: '0 auto', color: '#273746', fontFamily: 'Georgia, "Times New Roman", serif' }}>
      <div style={{ background: '#263d52', color: '#fff', margin: '-20px -20px 20px', padding: '18px 24px 0', boxShadow: '0 2px 8px rgba(21, 39, 53, 0.18)' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 16, alignItems: 'center', flexWrap: 'wrap' }}>
          <div>
            <div style={{ color: '#d8e3eb', fontFamily: 'Arial, sans-serif', fontSize: 12, fontWeight: 700, letterSpacing: 1.4, textTransform: 'uppercase' }}>Weather Underground</div>
            <h1 style={{ margin: '5px 0 3px', fontSize: 30, fontWeight: 500 }}>Jachenhausen</h1>
            <p style={{ margin: 0, color: '#bfccd6', fontFamily: 'Arial, sans-serif', fontSize: 13 }}>{observation?.stationName || `Persönliche Wetterstation ${settings.stationId}`}</p>
          </div>
          <div style={{ display: 'flex', gap: 8, fontFamily: 'Arial, sans-serif' }}>
            <button type="button" onClick={refresh} disabled={refreshing} style={{ ...buttonStyle('#fff', '#263d52'), borderColor: '#fff', opacity: refreshing ? 0.6 : 1 }}>↻ Aktualisieren</button>
            <button type="button" onClick={() => setSettingsOpen(true)} style={{ ...buttonStyle('transparent', '#fff'), borderColor: '#91a6b7' }}>⚙ Einstellungen</button>
          </div>
        </div>
        <StationStatusBar observation={displayObservation} configured={Boolean(data?.configured)} lastFetchAt={data?.lastFetchAt} />
        <div role="tablist" aria-label="Wetteransichten" style={{ display: 'flex', gap: 0, marginTop: 20, fontFamily: 'Arial, sans-serif', fontSize: 13 }}>
          <WeatherTabButton active={activeTab === 'today'} onClick={() => setActiveTab('today')}>TODAY</WeatherTabButton>
          <WeatherTabButton active={activeTab === 'hourly'} onClick={() => setActiveTab('hourly')}>HOURLY FORECAST</WeatherTabButton>
          <WeatherTabButton active={activeTab === 'tenDay'} onClick={() => setActiveTab('tenDay')}>10-DAY FORECAST</WeatherTabButton>
        </div>
      </div>

      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 16, alignItems: 'center', flexWrap: 'wrap', marginBottom: 14, fontFamily: 'Arial, sans-serif' }}>
        <div>
          <div style={{ color: '#768797', fontSize: 12, fontWeight: 700, letterSpacing: 1, textTransform: 'uppercase' }}>Persönliche Wetterstation</div>
          <div style={{ marginTop: 3, color: '#526575', fontSize: 14 }}>Station {settings.stationId} · {formatTime(observation?.observedAt)}</div>
        </div>
      </div>

      {message && <div style={{ ...panel, padding: '12px 16px', marginBottom: 16, color: '#2f6374', fontFamily: 'Arial, sans-serif' }}>{message}</div>}
      {data?.error && <div style={{ ...panel, padding: '14px 16px', marginBottom: 16, borderColor: '#efb8b4', color: '#a43835', fontFamily: 'Arial, sans-serif' }}>{data.error}</div>}

      {activeTab === 'today' ? (
        <>
          <section style={{ ...panel, padding: '26px 28px 22px', display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: 26, alignItems: 'center', background: '#fff' }}>
            <div style={{ display: 'flex', justifyContent: 'center' }}>
              <div style={{ width: 190, height: 190, borderRadius: '50%', border: '5px solid #62b94f', boxSizing: 'border-box', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', color: '#263d52', fontFamily: 'Arial, sans-serif' }}>
                <div style={{ color: '#607382', fontSize: 16, marginBottom: 4 }}>—° <span style={{ color: '#b8c1c7' }}>|</span> {formatNumber(displayObservation.feelsLike, 0)}°</div>
                <div style={{ color: '#69b82f', fontSize: 76, lineHeight: 0.95, fontWeight: 400 }}>{formatNumber(displayObservation.temperature, 0)}<span style={{ fontSize: 28, verticalAlign: 'top', marginLeft: 3 }}>{displayObservation.temperatureUnit}</span></div>
                <div style={{ color: '#263d52', fontSize: 15, marginTop: 8 }}>LIKE {formatNumber(displayObservation.feelsLike, 0)}°</div>
              </div>
            </div>
            <div style={{ display: 'grid', justifyItems: 'center', gap: 14, fontFamily: 'Arial, sans-serif' }}>
              <div style={{ color: '#263d52', fontSize: 70, lineHeight: 0.8, fontFamily: 'Georgia, "Times New Roman", serif' }}>☾</div>
              <div style={{ color: '#263d52', fontSize: 14 }}>Aktuelle Bedingungen</div>
              <div style={{ width: 62, height: 62, border: '3px solid #8d959a', borderRadius: '50%', display: 'grid', placeItems: 'center', color: '#263d52', fontWeight: 700, position: 'relative' }}>
                <span style={{ position: 'absolute', top: 5, fontSize: 11 }}>N</span>
                <span>{displayObservation.windDirection || '0'}</span>
              </div>
              <div style={{ color: '#607382', fontSize: 13 }}>{formatNumber(displayObservation.windSpeed)} {displayObservation.windUnit}</div>
            </div>
          </section>

          <section style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))', gap: 0, marginTop: 14, ...panel, overflow: 'hidden' }}>
            <Metric label="Luftfeuchte" value={`${formatNumber(displayObservation.humidity, 0)} %`} />
            <Metric label="Taupunkt" value={`${formatNumber(displayObservation.dewPoint)}${displayObservation.temperatureUnit}`} />
            <Metric label="UV-Index" value={formatNumber(displayObservation.uv, 0)} />
            <Metric label="Luftdruck" value={`${formatNumber(displayObservation.pressure)} ${displayObservation.pressureUnit}`} />
          </section>

          <section style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(210px, 1fr))', gap: 12, marginTop: 14 }}>
            <MetricCard title="Wind & Böen" value={`${formatNumber(displayObservation.windSpeed)} ${displayObservation.windUnit}`} detail={`${displayObservation.windDirection || '—'} · Böen ${formatNumber(displayObservation.windGust)} ${displayObservation.windUnit}`} />
            <MetricCard title="Luftdruck" value={`${formatNumber(displayObservation.pressure)} ${displayObservation.pressureUnit}`} detail={`Station ${displayObservation.stationId}`} />
            <MetricCard title="Niederschlag" value={`${formatNumber(displayObservation.precipRate)} ${displayObservation.precipUnit}/h`} detail={`Summe ${formatNumber(displayObservation.precipTotal)} ${displayObservation.precipUnit}`} />
            <MetricCard title="Solarstrahlung" value={`${formatNumber(displayObservation.solarRadiation, 0)} W/m²`} detail={`Höhe ${formatNumber(displayObservation.elevation, 0)} m`} />
          </section>

          <section style={{ ...panel, marginTop: 14, padding: 15, display: 'flex', justifyContent: 'space-between', gap: 14, flexWrap: 'wrap', color: '#6a7b80', fontFamily: 'Arial, sans-serif', fontSize: 12 }}>
            <span>Koordinaten {formatNumber(displayObservation.latitude, 3)}° N · {formatNumber(displayObservation.longitude, 3)}° E</span>
            <span>Letzter Abruf: {formatTime(data?.lastFetchAt)}</span>
          </section>
        </>
      ) : activeTab === 'hourly' ? (
        <ForecastUnavailable title="Hourly Forecast" observation={displayObservation} />
      ) : activeTab === 'tenDay' ? (
        <ForecastUnavailable title="10-Day Forecast" observation={displayObservation} />
      ) : (
        <div style={{ ...panel, padding: 32, textAlign: 'center', color: '#6a7b80', fontFamily: 'Arial, sans-serif' }}>Keine Wetteransicht ausgewählt.</div>
      )}

      {settingsOpen && <SettingsModal settings={settings} setSettings={setSettings} saving={saving} onClose={() => setSettingsOpen(false)} onSave={saveSettings} />}
    </div>
  )
}

function WeatherTabButton({ active, children, onClick }: { active: boolean; children: string; onClick: () => void }) {
  return <button type="button" role="tab" aria-selected={active} onClick={onClick} style={{ border: 0, borderTop: active ? '3px solid #d7463f' : '3px solid transparent', borderLeft: '1px solid rgba(255,255,255,0.08)', background: active ? '#fff' : 'transparent', color: active ? '#263d52' : '#bdcbd5', padding: '12px 20px 11px', fontWeight: 700, cursor: 'pointer' }}>{children}</button>
}

function StationStatusBar({ observation, configured, lastFetchAt }: { observation?: Observation; configured: boolean; lastFetchAt?: string }) {
  return <div style={{ display: 'flex', alignItems: 'center', gap: 18, flexWrap: 'wrap', marginTop: 18, padding: '11px 0 2px', borderTop: '1px solid rgba(216, 227, 235, 0.22)', fontFamily: 'Arial, sans-serif', fontSize: 13 }}>
    <span style={{ color: configured ? '#fff' : '#ffcfca', fontWeight: 700 }}>{configured ? 'STATION ONLINE' : 'STATION NICHT EINGERICHTET'}</span>
    {observation ? <>
      <span style={{ color: '#fff', fontSize: 22, fontWeight: 700 }}>{formatNumber(observation.temperature)}{observation.temperatureUnit}</span>
      <span style={{ color: '#c5d2dc' }}>Gefühlt {formatNumber(observation.feelsLike)}{observation.temperatureUnit}</span>
      <span style={{ color: '#c5d2dc' }}>Feuchte {formatNumber(observation.humidity, 0)} %</span>
      <span style={{ color: '#c5d2dc' }}>Wind {formatNumber(observation.windSpeed)} {observation.windUnit}</span>
      <span style={{ color: '#9fb1bf', marginLeft: 'auto' }}>Update {formatTime(lastFetchAt || observation.updatedAt)}</span>
    </> : <span style={{ color: '#c5d2dc' }}>Noch keine Messdaten verfügbar</span>}
  </div>
}

function ForecastUnavailable({ title, observation }: { title: string; observation: Observation }) {
  return <section style={{ background: '#fff', border: '1px solid #d9e0e5', borderRadius: 4, boxShadow: '0 3px 12px rgba(27, 49, 67, 0.08)', fontFamily: 'Arial, sans-serif' }}>
    <div style={{ padding: '14px 18px', background: '#eef1f3', borderBottom: '1px solid #d9e0e5', color: '#263d52', fontSize: 14, fontWeight: 700 }}>{title}</div>
    <div style={{ padding: 28, textAlign: 'center' }}>
      <div style={{ color: '#263d52', fontFamily: 'Georgia, "Times New Roman", serif', fontSize: 26, marginBottom: 8 }}>Forecast data unavailable</div>
      <p style={{ maxWidth: 560, margin: '0 auto 22px', color: '#687b8a', lineHeight: 1.6 }}>Die angebundene Personal Weather Station liefert aktuell Messwerte, aber keine Stunden- oder 10-Tage-Prognose.</p>
      <div style={{ display: 'inline-flex', gap: 26, flexWrap: 'wrap', justifyContent: 'center', padding: '14px 20px', background: '#f7f9fa', border: '1px solid #e1e7eb', color: '#526575', fontSize: 13 }}>
        <span>Aktuell <strong>{formatNumber(observation.temperature)}{observation.temperatureUnit}</strong></span>
        <span>Gefühlt <strong>{formatNumber(observation.feelsLike)}{observation.temperatureUnit}</strong></span>
        <span>Feuchte <strong>{formatNumber(observation.humidity, 0)} %</strong></span>
      </div>
    </div>
  </section>
}

function buttonStyle(background: string, color: string): CSSProperties {
  return { border: '1px solid #b7d7d2', borderRadius: 3, background, color, padding: '9px 13px', fontWeight: 800, cursor: 'pointer' }
}

function Metric({ label, value }: { label: string; value: string }) {
  return <div style={{ flex: '1 1 130px', padding: '22px 16px', borderLeft: '1px solid #e1e7eb', display: 'flex', flexDirection: 'column', justifyContent: 'center' }}><div style={{ color: '#718392', fontFamily: 'Arial, sans-serif', fontSize: 12, marginBottom: 9 }}>{label}</div><div style={{ fontSize: 24, fontWeight: 700, color: '#304c62' }}>{value}</div></div>
}

function MetricCard({ title, value, detail }: { title: string; value: string; detail: string }) {
  return <div style={{ background: '#fff', border: '1px solid #d9e0e5', borderRadius: 4, padding: 17, boxShadow: '0 3px 12px rgba(27, 49, 67, 0.06)' }}><div style={{ color: '#718392', fontFamily: 'Arial, sans-serif', fontSize: 12, fontWeight: 700, textTransform: 'uppercase' }}>{title}</div><div style={{ color: '#304c62', fontSize: 24, fontWeight: 700, margin: '10px 0 6px' }}>{value}</div><div style={{ color: '#71858a', fontFamily: 'Arial, sans-serif', fontSize: 12 }}>{detail}</div></div>
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
