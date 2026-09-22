import type { CSSProperties, FormEvent } from 'react'
import React, { useEffect, useState } from 'react'
import PageHeader, { pageHeaderButtonStyle } from '../components/PageHeader'

const API = '/api/enyaq'

export type EnyaqConfig = {
  hasToken: boolean
  tokenPreview: string
  vin: string
  apiUrl: string
  pollIntervalMinutes: number
  autoPollEnabled: boolean
  lastConfigUpdate?: string
}

export type EnyaqVehicleData = {
  batteryLevelPct: number
  targetSoCPct: number
  remainingRangeKm: number
  chargingState: string
  chargePowerKw: number
  chargeRateKmPerHour: number
  remainingChargingTimeMin: number
  chargingMode: string
  plugConnectionState: string
  plugLockState: string
  mileageKm: number
  lockState: string
  doorsClosed: boolean
  windowsClosed: boolean
  trunkClosed: boolean
  bonnetClosed: boolean
  lightsOn: boolean
  targetTemperatureC: number
  climatisationState: string
  windowHeatingFront: boolean
  windowHeatingRear: boolean
  outdoorTemperatureC: number
  modelName: string
  lastUpdated: string
  lastPollStatus: string
  lastError?: string
  rawPayload?: string
}

export type HistoryEntry = {
  timestamp: string
  batteryLevelPct: number
  remainingRangeKm: number
  chargingState: string
  chargePowerKw: number
  mileageKm: number
  lockState: string
}

export type EnyaqStatusResponse = {
  config: EnyaqConfig
  data: EnyaqVehicleData
  history: HistoryEntry[]
  serverTime: string
  nextPollInSec: number
  pollIntervalMin: number
}

type Feedback = {
  type: 'success' | 'error'
  msg: string
}

export default function EnyaqPage() {
  const [status, setStatus] = useState<EnyaqStatusResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [fetching, setFetching] = useState(false)
  const [feedback, setFeedback] = useState<Feedback | null>(null)

  // Token / Settings Modal & Form State
  const [modalOpen, setModalOpen] = useState(false)
  const [tokenInput, setTokenInput] = useState('')
  const [vinInput, setVinInput] = useState('')
  const [apiUrlInput, setApiUrlInput] = useState('')
  const [pollIntervalInput, setPollIntervalInput] = useState(10)
  const [autoPollInput, setAutoPollInput] = useState(true)
  const [showRawPayload, setShowRawPayload] = useState(false)
  const [savingConfig, setSavingConfig] = useState(false)

  const [photoView, setPhotoView] = useState<'perspective' | 'side' | 'front'>('perspective')

  const loadStatus = async () => {
    try {
      const res = await fetch(`${API}/status`)
      if (res.ok) {
        const data: EnyaqStatusResponse = await res.json()
        setStatus(data)
      }
    } catch {
      // transient network error
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadStatus()
    const timer = setInterval(loadStatus, 5000)
    return () => clearInterval(timer)
  }, [])

  const openConfigModal = () => {
    if (status?.config) {
      setVinInput(status.config.vin || '')
      setApiUrlInput(status.config.apiUrl || '')
      setPollIntervalInput(status.config.pollIntervalMinutes || 10)
      setAutoPollInput(status.config.autoPollEnabled)
    }
    setTokenInput('')
    setFeedback(null)
    setModalOpen(true)
  }

  const handleSaveConfig = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setSavingConfig(true)
    setFeedback(null)

    try {
      const body: Record<string, unknown> = {
        vin: vinInput.trim(),
        apiUrl: apiUrlInput.trim(),
        pollIntervalMinutes: Number(pollIntervalInput) || 10,
        autoPollEnabled: autoPollInput,
      }
      if (tokenInput.trim() !== '') {
        body.apiToken = tokenInput.trim()
      }

      const res = await fetch(`${API}/config`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })

      const data = await res.json()
      if (res.ok && data.ok) {
        setFeedback({ type: 'success', msg: '✅ Konfiguration & API-Token sicher in Datenbank gespeichert!' })
        setModalOpen(false)
        setTokenInput('')
        await loadStatus()
      } else {
        setFeedback({ type: 'error', msg: `❌ Fehler: ${data.error || 'Speichern fehlgeschlagen'}` })
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      setFeedback({ type: 'error', msg: `❌ Fehler: ${message}` })
    } finally {
      setSavingConfig(false)
    }
  }

  const triggerManualFetch = async () => {
    setFetching(true)
    setFeedback(null)
    try {
      const res = await fetch(`${API}/fetch`, {
        method: 'POST',
      })
      const data = await res.json()
      if (res.ok && data.ok) {
        setFeedback({ type: 'success', msg: '✅ Fahrzeugdaten erfolgreich aktualisiert und in MQTT gepublisht!' })
        await loadStatus()
      } else {
        setFeedback({ type: 'error', msg: `❌ ${data.error || 'Abfrage fehlgeschlagen'}` })
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      setFeedback({ type: 'error', msg: `❌ Fehler: ${message}` })
    } finally {
      setFetching(false)
    }
  }

  const formatTimestamp = (isoString?: string) => {
    if (!isoString) return '—'
    try {
      const d = new Date(isoString)
      if (isNaN(d.getTime())) return isoString
      return new Intl.DateTimeFormat('de-DE', {
        dateStyle: 'medium',
        timeStyle: 'medium',
      }).format(d)
    } catch {
      return isoString
    }
  }

  const formatNextPoll = (sec: number) => {
    if (sec <= 0) return 'Jetzt / Bald'
    const mins = Math.floor(sec / 60)
    const remSec = sec % 60
    return `${mins}m ${remSec}s`
  }

  const data = status?.data
  const config = status?.config
  const batteryPct = data?.batteryLevelPct ?? 0
  const isCharging = data?.chargingState === 'charging' || (data?.chargePowerKw ?? 0) > 0.05
  const isConnected = data?.plugConnectionState === 'connected'

  // Battery status color calculation
  const getBatteryColor = (pct: number) => {
    if (pct <= 20) return '#ef4444' // Red
    if (pct <= 45) return '#f59e0b' // Orange / Amber
    if (pct <= 80) return '#10b981' // Green
    return '#059669' // Emerald
  }

  const cardStyle: CSSProperties = {
    background: '#ffffff',
    borderRadius: 14,
    padding: '20px 24px',
    boxShadow: '0 2px 8px rgba(0,0,0,0.06)',
    border: '1px solid #e2e8f0',
  }

  return (
    <div style={{ maxWidth: 1100, margin: '0 auto' }}>
      <PageHeader
        eyebrow="ŠKODA CONNECT"
        title={data?.modelName || 'Škoda Enyaq'}
        subtitle="Fahrzeugdaten, Batterie, Reichweite und Ladezustand"
        actions={<div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', fontFamily: 'Arial, sans-serif' }}>
          <button
            type="button"
            onClick={triggerManualFetch}
            disabled={fetching}
            style={{ ...pageHeaderButtonStyle('#fff', '#263d52'), borderColor: '#fff', opacity: fetching ? 0.6 : 1 }}
            title="Jetzt sofort telemetry abfragen"
          >
            {fetching ? '⟳ Aktualisiere …' : '↻ Aktualisieren'}
          </button>

          <button
            type="button"
            onClick={openConfigModal}
            style={{ ...pageHeaderButtonStyle('transparent', '#fff'), borderColor: '#91a6b7' }}
          >
            ⚙ Einstellungen
          </button>
        </div>}
      />

      {/* Feedback Banner */}
      {feedback && (
        <div style={{
          padding: '12px 18px',
          borderRadius: 8,
          marginBottom: 18,
          background: feedback.type === 'success' ? '#d1fae5' : '#fee2e2',
          color: feedback.type === 'success' ? '#065f46' : '#991b1b',
          fontWeight: 600,
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
        }}>
          <div>{feedback.msg}</div>
          <button
            type="button"
            onClick={() => setFeedback(null)}
            style={{ background: 'transparent', border: 'none', cursor: 'pointer', fontWeight: 700, fontSize: 16 }}
          >
            ✕
          </button>
        </div>
      )}

      {/* Warning when no token is present */}
      {!loading && !config?.hasToken && (
        <div style={{
          ...cardStyle,
          marginBottom: 20,
          background: '#fffbeb',
          border: '1px solid #fde68a',
          color: '#92400e',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          flexWrap: 'wrap',
          gap: 12,
        }}>
          <div>
            <strong>⚠️ Kein API-Token hinterlegt:</strong> Bitte klicke auf "⚙️ API-Token & Einstellungen", um deinen Token einzutragen.
          </div>
          <button
            type="button"
            onClick={openConfigModal}
            style={{ ...pageHeaderButtonStyle('#d97706', '#fff'), padding: '8px 14px', fontSize: 13 }}
          >
            Token jetzt eingeben
          </button>
        </div>
      )}

      {/* Hero: Blue Škoda Enyaq Coupé RS with Live Data Badges */}
      <div style={{ ...cardStyle, marginBottom: 22, padding: '20px 24px', background: 'linear-gradient(180deg, #ffffff 0%, #f0f7ff 100%)', position: 'relative' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12, flexWrap: 'wrap', gap: 8 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <span style={{ fontSize: 22 }}>🚙</span>
            <div>
              <h3 style={{ margin: 0, fontSize: 18, color: '#0f2744', fontWeight: 800 }}>
                {data?.modelName || 'Škoda Enyaq Coupé RS iV'} — Race Blau Metallic
              </h3>
              <span style={{ fontSize: 12, color: '#64748b' }}>
                Original 1:43 Coupé RS Modellansicht mit Live-Telemetrie-Overlays
              </span>
            </div>
          </div>

          <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
            {/* Perspektiven-Auswahl */}
            <div style={{ display: 'inline-flex', background: '#e2e8f0', borderRadius: 8, padding: 3, gap: 4 }}>
              <button
                type="button"
                onClick={() => setPhotoView('perspective')}
                style={{
                  padding: '5px 12px',
                  borderRadius: 6,
                  border: 'none',
                  fontSize: 12,
                  fontWeight: 600,
                  cursor: 'pointer',
                  background: photoView === 'perspective' ? '#0f172a' : 'transparent',
                  color: photoView === 'perspective' ? '#ffffff' : '#475569',
                  transition: 'all 0.15s ease',
                }}
              >
                3/4 Ansicht
              </button>
              <button
                type="button"
                onClick={() => setPhotoView('side')}
                style={{
                  padding: '5px 12px',
                  borderRadius: 6,
                  border: 'none',
                  fontSize: 12,
                  fontWeight: 600,
                  cursor: 'pointer',
                  background: photoView === 'side' ? '#0f172a' : 'transparent',
                  color: photoView === 'side' ? '#ffffff' : '#475569',
                  transition: 'all 0.15s ease',
                }}
              >
                Seite
              </button>
              <button
                type="button"
                onClick={() => setPhotoView('front')}
                style={{
                  padding: '5px 12px',
                  borderRadius: 6,
                  border: 'none',
                  fontSize: 12,
                  fontWeight: 600,
                  cursor: 'pointer',
                  background: photoView === 'front' ? '#0f172a' : 'transparent',
                  color: photoView === 'front' ? '#ffffff' : '#475569',
                  transition: 'all 0.15s ease',
                }}
              >
                Front
              </button>
            </div>

            <span style={{
              fontSize: 12,
              padding: '4px 10px',
              borderRadius: 999,
              background: isCharging ? '#dcfce7' : isConnected ? '#e0e7ff' : '#f1f5f9',
              color: isCharging ? '#15803d' : isConnected ? '#3730a3' : '#475569',
              fontWeight: 700,
              border: `1px solid ${isCharging ? '#86efac' : isConnected ? '#c7d2fe' : '#cbd5e1'}`,
              display: 'flex',
              alignItems: 'center',
              gap: 6,
            }}>
              <span style={{
                width: 8,
                height: 8,
                borderRadius: '50%',
                background: isCharging ? '#16a34a' : isConnected ? '#4f46e5' : '#94a3b8',
                boxShadow: isCharging ? '0 0 8px #22c55e' : 'none',
              }} />
              {isCharging ? `LÄDT (${data?.chargePowerKw?.toFixed(1) || 0} kW)` : isConnected ? 'VERBUNDEN (STANDBY)' : 'PARKEND'}
            </span>
          </div>
        </div>

        {/* High-Res Photo with Live Overlay Badges */}
        <EnyaqPhotoVisual
          data={data}
          batteryPct={batteryPct}
          isCharging={isCharging}
          isConnected={isConnected}
          getBatteryColor={getBatteryColor}
          photoView={photoView}
        />
      </div>

      {/* Main Grid Overview */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: 18, marginBottom: 20 }}>
        {/* Card 1: Batterie & Reichweite */}
        <div style={{ ...cardStyle, position: 'relative', overflow: 'hidden' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 14 }}>
            <h3 style={{ margin: 0, fontSize: 17, color: '#334155' }}>🔋 Batterie & Reichweite</h3>
            <span style={{
              fontSize: 12,
              padding: '3px 8px',
              borderRadius: 6,
              background: isCharging ? '#dcfce7' : '#f1f5f9',
              color: isCharging ? '#166534' : '#475569',
              fontWeight: 700,
            }}>
              {isCharging ? '⚡ WIRD GELADEN' : 'STANDBY'}
            </span>
          </div>

          <div style={{ display: 'flex', alignItems: 'baseline', gap: 12, marginBottom: 10 }}>
            <span style={{ fontSize: 44, fontWeight: 800, color: getBatteryColor(batteryPct) }}>
              {batteryPct.toFixed(0)}%
            </span>
            <span style={{ fontSize: 20, fontWeight: 700, color: '#334155' }}>
              ca. {data?.remainingRangeKm ? `${data.remainingRangeKm.toFixed(0)} km` : '—'}
            </span>
          </div>

          {/* Progress Bar SoC */}
          <div style={{ width: '100%', height: 14, background: '#e2e8f0', borderRadius: 999, overflow: 'hidden', marginBottom: 16 }}>
            <div style={{
              width: `${Math.min(100, Math.max(0, batteryPct))}%`,
              height: '100%',
              background: `linear-gradient(90deg, ${getBatteryColor(batteryPct)}, #10b981)`,
              borderRadius: 999,
              transition: 'width 0.5s ease',
            }} />
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10, fontSize: 13, color: '#475569' }}>
            <div>Ladelimit (Ziel): <strong>{data?.targetSoCPct ? `${data.targetSoCPct}%` : '80%'}</strong></div>
            <div>Kilometerstand: <strong>{data?.mileageKm ? `${data.mileageKm.toLocaleString('de-DE')} km` : '—'}</strong></div>
          </div>
        </div>

        {/* Card 2: Ladestatus & Leistung */}
        <div style={{ ...cardStyle }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 14 }}>
            <h3 style={{ margin: 0, fontSize: 17, color: '#334155' }}>⚡ Ladevorgang</h3>
            <span style={{
              fontSize: 12,
              padding: '3px 8px',
              borderRadius: 6,
              background: isConnected ? '#e0e7ff' : '#f1f5f9',
              color: isConnected ? '#3730a3' : '#64748b',
              fontWeight: 700,
            }}>
              {isConnected ? '🔌 KABEL VERBUNDEN' : '🔌 NICHT VERBUNDEN'}
            </span>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
            <StatusMetricBox
              label="Ladeleistung"
              value={data?.chargePowerKw !== undefined ? `${data.chargePowerKw.toFixed(2)} kW` : '0.0 kW'}
              highlight={isCharging}
            />
            <StatusMetricBox
              label="Laderate"
              value={data?.chargeRateKmPerHour ? `${data.chargeRateKmPerHour.toFixed(0)} km/h` : '0 km/h'}
            />
            <StatusMetricBox
              label="Restladezeit"
              value={data?.remainingChargingTimeMin ? `${data.remainingChargingTimeMin} min` : '—'}
            />
            <StatusMetricBox
              label="Lademodus"
              value={data?.chargingMode ? data.chargingMode.toUpperCase() : 'AC'}
            />
          </div>

          <div style={{ marginTop: 12, fontSize: 12, color: '#64748b' }}>
            Steckerverriegelung: <strong>{data?.plugLockState === 'locked' ? '🔒 Verriegelt' : '🔓 Entriegelt'}</strong>
          </div>
        </div>

        {/* Card 3: Fahrzeugzustand & Klima */}
        <div style={{ ...cardStyle }}>
          <h3 style={{ margin: '0 0 14px', fontSize: 17, color: '#334155' }}>🛡️ Fahrzeugstatus & Klima</h3>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10, fontSize: 13, marginBottom: 14 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <span>{data?.lockState === 'locked' ? '🔒' : '🔓'}</span>
              <span>Schlösser: <strong>{data?.lockState === 'locked' ? 'Verriegelt' : 'Offen'}</strong></span>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <span>{data?.doorsClosed !== false ? '🚪' : '⚠️'}</span>
              <span>Türen: <strong>{data?.doorsClosed !== false ? 'Geschlossen' : 'Offen'}</strong></span>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <span>{data?.windowsClosed !== false ? '🪟' : '⚠️'}</span>
              <span>Fenster: <strong>{data?.windowsClosed !== false ? 'Geschlossen' : 'Offen'}</strong></span>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <span>{data?.trunkClosed !== false ? '📦' : '⚠️'}</span>
              <span>Kofferraum: <strong>{data?.trunkClosed !== false ? 'Zu' : 'Auf'}</strong></span>
            </div>
          </div>

          <div style={{ borderTop: '1px solid #f1f5f9', paddingTop: 12, display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, fontSize: 13 }}>
            <div>Zieltemperatur: <strong>{data?.targetTemperatureC ? `${data.targetTemperatureC.toFixed(1)} °C` : '21.0 °C'}</strong></div>
            <div>Außentemperatur: <strong>{data?.outdoorTemperatureC !== undefined ? `${data.outdoorTemperatureC.toFixed(1)} °C` : '—'}</strong></div>
            <div>Klimatisierung: <strong>{data?.climatisationState || 'Aus'}</strong></div>
            <div>Scheibenheizung: <strong>{data?.windowHeatingFront ? 'Ein' : 'Aus'}</strong></div>
          </div>
        </div>
      </div>

      {/* Sync / MQTT Status Card */}
      <div style={{ ...cardStyle, marginBottom: 20 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 12 }}>
          <div>
            <h3 style={{ margin: 0, fontSize: 16, color: '#334155' }}>📡 Synchronisation & MQTT Status</h3>
            <p style={{ margin: '4px 0 0', color: '#64748b', fontSize: 13 }}>
              Automatische Abfrage alle <strong>{status?.pollIntervalMin || 10} Minuten</strong> in Go & Veröffentlichung unter Topic <code>enyaq/*</code>
            </p>
          </div>

          <div style={{ display: 'flex', gap: 14, fontSize: 13, color: '#475569', flexWrap: 'wrap' }}>
            <div>Letzter Abruf: <strong>{formatTimestamp(data?.lastUpdated)}</strong></div>
            <div>Nächste Abfrage in: <strong>{formatNextPoll(status?.nextPollInSec ?? 0)}</strong></div>
            <div>Status: <strong style={{ color: data?.lastPollStatus === 'ok' ? '#059669' : data?.lastPollStatus === 'error' ? '#dc2626' : '#d97706' }}>
              {data?.lastPollStatus === 'ok' ? '✅ Erfolgreich' : data?.lastPollStatus === 'error' ? '❌ Fehler' : '⏸️ Wartend'}
            </strong></div>
          </div>
        </div>

        {data?.lastError && (
          <div style={{ marginTop: 12, padding: 10, background: '#fee2e2', color: '#991b1b', borderRadius: 8, fontSize: 12 }}>
            <strong>Letzter Fehler:</strong> {data.lastError}
          </div>
        )}
      </div>

      {/* Telemetry History Table */}
      <div style={{ ...cardStyle, marginBottom: 20 }}>
        <h3 style={{ margin: '0 0 14px', fontSize: 16, color: '#334155' }}>📜 Verlauf der letzten Messungen (bbolt Datenbank)</h3>
        {status?.history && status.history.length > 0 ? (
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13, color: '#334155' }}>
              <thead>
                <tr style={{ background: '#f8fafc', borderBottom: '1px solid #e2e8f0' }}>
                  <th style={{ textAlign: 'left', padding: '8px 12px' }}>Zeitpunkt</th>
                  <th style={{ textAlign: 'left', padding: '8px 12px' }}>Batterie</th>
                  <th style={{ textAlign: 'left', padding: '8px 12px' }}>Reichweite</th>
                  <th style={{ textAlign: 'left', padding: '8px 12px' }}>Ladeleistung</th>
                  <th style={{ textAlign: 'left', padding: '8px 12px' }}>Ladestatus</th>
                  <th style={{ textAlign: 'left', padding: '8px 12px' }}>Kilometer</th>
                </tr>
              </thead>
              <tbody>
                {[...status.history].reverse().slice(0, 10).map((h, i) => (
                  <tr key={i} style={{ borderBottom: '1px solid #f1f5f9' }}>
                    <td style={{ padding: '8px 12px' }}>{formatTimestamp(h.timestamp)}</td>
                    <td style={{ padding: '8px 12px', fontWeight: 700, color: getBatteryColor(h.batteryLevelPct) }}>{h.batteryLevelPct}%</td>
                    <td style={{ padding: '8px 12px' }}>{h.remainingRangeKm} km</td>
                    <td style={{ padding: '8px 12px' }}>{h.chargePowerKw > 0 ? `${h.chargePowerKw.toFixed(1)} kW` : '—'}</td>
                    <td style={{ padding: '8px 12px' }}>{h.chargingState || '—'}</td>
                    <td style={{ padding: '8px 12px' }}>{h.mileageKm ? `${h.mileageKm.toLocaleString('de-DE')} km` : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div style={{ color: '#94a3b8', fontSize: 13 }}>Noch keine Verlaufseinträge in der Datenbank vorhanden.</div>
        )}
      </div>

      {/* Raw Payload Debug Box (Collapsible) */}
      <div style={{ marginTop: 10 }}>
        <button
          type="button"
          onClick={() => setShowRawPayload(p => !p)}
          style={{ background: 'transparent', border: 'none', color: '#64748b', fontSize: 12, cursor: 'pointer', padding: 0 }}
        >
          {showRawPayload ? '▲ Raw JSON Antwort ausblenden' : '▼ Raw JSON Antwort anzeigen (Debug)'}
        </button>
        {showRawPayload && (
          <pre style={{
            background: '#1e293b',
            color: '#f8fafc',
            padding: 14,
            borderRadius: 8,
            fontSize: 11,
            maxHeight: 250,
            overflowY: 'auto',
            marginTop: 8,
          }}>
            {data?.rawPayload || 'Kein Raw-Payload vorhanden'}
          </pre>
        )}
      </div>

      {/* Modal / Popup for API-Token & Settings */}
      {modalOpen && (
        <div style={{
          position: 'fixed',
          inset: 0,
          background: 'rgba(15, 23, 42, 0.65)',
          backdropFilter: 'blur(3px)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          padding: 16,
          zIndex: 1000,
        }}>
          <div style={{
            background: '#ffffff',
            borderRadius: 16,
            width: '100%',
            maxWidth: 580,
            boxShadow: '0 20px 25px -5px rgba(0, 0, 0, 0.2), 0 10px 10px -5px rgba(0, 0, 0, 0.1)',
            overflow: 'hidden',
          }}>
            {/* Modal Header */}
            <div style={{
              padding: '18px 24px',
              borderBottom: '1px solid #e2e8f0',
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              background: '#f8fafc',
            }}>
              <h2 style={{ margin: 0, fontSize: 18, color: '#0f172a' }}>
                🔑 Škoda API-Token & Einstellungen
              </h2>
              <button
                type="button"
                onClick={() => setModalOpen(false)}
                style={{ background: 'transparent', border: 'none', fontSize: 20, cursor: 'pointer', color: '#64748b' }}
              >
                ✕
              </button>
            </div>

            {/* Modal Body Form */}
            <form onSubmit={handleSaveConfig} style={{ padding: '20px 24px' }}>
              <p style={{ margin: '0 0 16px', color: '#64748b', fontSize: 13 }}>
                Hier kannst du deinen API-Token (z. B. MyŠkoda Bearer Token, Connect Token oder Gateway URL) eintragen.
                Alle Einstellungen werden dauerhaft in der bbolt-Datenbank gespeichert und überleben Serverneustarts.
              </p>

              {/* Token Field */}
              <div style={{ marginBottom: 16 }}>
                <label style={{ display: 'block', fontWeight: 600, fontSize: 13, color: '#334155', marginBottom: 6 }}>
                  MyŠkoda API-Key / Token:
                </label>
                <input
                  type="password"
                  placeholder={config?.hasToken ? `Aktueller API-Key aktiv (${config.tokenPreview}) - leer lassen für unverändert` : 'MyŠkoda API-Key eingeben (z.B. aus der MyŠkoda App / go.skoda.eu/api-keys)'}
                  value={tokenInput}
                  onChange={e => setTokenInput(e.target.value)}
                  style={{
                    width: '100%',
                    padding: '10px 12px',
                    borderRadius: 8,
                    border: '1px solid #cbd5e1',
                    fontSize: 13,
                    boxSizing: 'border-box',
                    fontFamily: 'monospace',
                  }}
                />
                <span style={{ fontSize: 11, color: '#64748b', display: 'block', marginTop: 4 }}>
                  {config?.hasToken ? `✅ API-Key gespeichert: ${config.tokenPreview}` : '❌ Noch kein API-Key gespeichert'}
                </span>
              </div>

              {/* VIN / FIN */}
              <div style={{ marginBottom: 16 }}>
                <label style={{ display: 'block', fontWeight: 600, fontSize: 13, color: '#334155', marginBottom: 6 }}>
                  Fahrgestellnummer (VIN / FIN, optional):
                </label>
                <input
                  type="text"
                  placeholder="TMB..."
                  value={vinInput}
                  onChange={e => setVinInput(e.target.value.toUpperCase())}
                  style={{
                    width: '100%',
                    padding: '10px 12px',
                    borderRadius: 8,
                    border: '1px solid #cbd5e1',
                    fontSize: 13,
                    boxSizing: 'border-box',
                    textTransform: 'uppercase',
                  }}
                />
              </div>

              {/* Custom API URL */}
              <div style={{ marginBottom: 16 }}>
                <label style={{ display: 'block', fontWeight: 600, fontSize: 13, color: '#334155', marginBottom: 6 }}>
                  Eigene API-URL / Proxy (optional):
                </label>
                <input
                  type="text"
                  placeholder="https://public.api.connect.skoda-auto.cz/api/v1/vehicles/... oder lokaler Proxy"
                  value={apiUrlInput}
                  onChange={e => setApiUrlInput(e.target.value)}
                  style={{
                    width: '100%',
                    padding: '10px 12px',
                    borderRadius: 8,
                    border: '1px solid #cbd5e1',
                    fontSize: 13,
                    boxSizing: 'border-box',
                  }}
                />
              </div>

              {/* Poll Interval & Auto-Poll */}
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14, marginBottom: 20 }}>
                <div>
                  <label style={{ display: 'block', fontWeight: 600, fontSize: 13, color: '#334155', marginBottom: 6 }}>
                    Abfrage-Intervall (Minuten):
                  </label>
                  <input
                    type="number"
                    min={1}
                    max={1440}
                    value={pollIntervalInput}
                    onChange={e => setPollIntervalInput(Number(e.target.value))}
                    style={{
                      width: '100%',
                      padding: '10px 12px',
                      borderRadius: 8,
                      border: '1px solid #cbd5e1',
                      fontSize: 13,
                      boxSizing: 'border-box',
                    }}
                  />
                </div>

                <div style={{ display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                  <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer', fontSize: 13, fontWeight: 600, color: '#334155' }}>
                    <input
                      type="checkbox"
                      checked={autoPollInput}
                      onChange={e => setAutoPollInput(e.target.checked)}
                      style={{ width: 18, height: 18 }}
                    />
                    Automatischer 10-Min Sync aktiv
                  </label>
                </div>
              </div>

              {/* Modal Actions */}
              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10 }}>
                <button
                  type="button"
                  onClick={() => setModalOpen(false)}
                  style={{ ...pageHeaderButtonStyle('#f3f7f7', '#31565c'), padding: '9px 16px' }}
                >
                  Abbrechen
                </button>
                <button
                  type="submit"
                  disabled={savingConfig}
                  style={{ ...pageHeaderButtonStyle('#0f766e', '#fff'), padding: '9px 18px', opacity: savingConfig ? 0.6 : 1 }}
                >
                  {savingConfig ? '💾 Speichere…' : '💾 In Datenbank speichern'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

function StatusMetricBox({ label, value, highlight = false }: { label: string; value: string; highlight?: boolean }) {
  return (
    <div style={{
      background: highlight ? '#ecfdf5' : '#f8fafc',
      border: highlight ? '1px solid #a7f3d0' : '1px solid #e2e8f0',
      borderRadius: 10,
      padding: '10px 14px',
    }}>
      <div style={{ fontSize: 11, color: highlight ? '#065f46' : '#64748b', marginBottom: 3, fontWeight: 600 }}>{label}</div>
      <div style={{ fontSize: 16, fontWeight: 700, color: highlight ? '#047857' : '#1e293b' }}>{value}</div>
    </div>
  )
}

/**
 * @brief Fotorealistische Visualisierung des originalen Škoda Enyaq Coupé RS in Race Blau mit direkt angebundenen Live-Werten.
 */
function EnyaqPhotoVisual({
  data,
  batteryPct,
  isCharging,
  isConnected,
  getBatteryColor,
  photoView,
}: {
  data?: EnyaqVehicleData
  batteryPct: number
  isCharging: boolean
  isConnected: boolean
  getBatteryColor: (pct: number) => string
  photoView: 'perspective' | 'side' | 'front'
}) {
  const rangeKm = data?.remainingRangeKm !== undefined ? Math.round(data.remainingRangeKm) : null
  const chargePower = data?.chargePowerKw !== undefined ? data.chargePowerKw.toFixed(1) : '0.0'
  const isLocked = data?.lockState === 'locked'
  const targetTemp = data?.targetTemperatureC ? data.targetTemperatureC.toFixed(1) : '21.0'
  const outdoorTemp = data?.outdoorTemperatureC !== undefined ? data.outdoorTemperatureC.toFixed(1) : null
  const mileage = data?.mileageKm ? data.mileageKm.toLocaleString('de-DE') : null

  const socColor = getBatteryColor(batteryPct)

  const photoSrc = photoView === 'side'
    ? '/enyaq-coupe-rs-blue-side.jpg'
    : photoView === 'front'
    ? '/enyaq-coupe-rs-blue-front.jpg'
    : '/enyaq-coupe-rs-blue.jpg'

  return (
    <div style={{ position: 'relative', width: '100%', borderRadius: 14, overflow: 'hidden', padding: '10px 0' }}>
      {/* Central High-Resolution Vehicle Photo */}
      <div style={{
        position: 'relative',
        maxWidth: 720,
        margin: '0 auto',
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
      }}>
        <img
          src={photoSrc}
          alt="Original Škoda Enyaq Coupé RS 1:43 Modellauto blau"
          style={{
            width: '100%',
            height: 'auto',
            maxHeight: 400,
            objectFit: 'contain',
            borderRadius: 12,
            filter: isCharging ? 'drop-shadow(0 12px 28px rgba(34, 197, 94, 0.28))' : 'drop-shadow(0 12px 24px rgba(2, 132, 199, 0.22))',
            transition: 'all 0.3s ease',
          }}
        />

        {/* Floating Live Telemetry Badge 1: Reichweite (Top Left / Front) */}
        <div style={{
          position: 'absolute',
          top: '8%',
          left: '2%',
          background: 'rgba(255, 255, 255, 0.95)',
          backdropFilter: 'blur(8px)',
          border: '1.5px solid #0284c7',
          borderRadius: 12,
          padding: '8px 14px',
          boxShadow: '0 6px 16px rgba(2, 132, 199, 0.18)',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'flex-start',
        }}>
          <span style={{ fontSize: 10, fontWeight: 700, color: '#64748b', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
            🛣️ Reichweite (Est.)
          </span>
          <span style={{ fontSize: 22, fontWeight: 800, color: '#0369a1', lineHeight: 1.1 }}>
            {rangeKm !== null ? `${rangeKm} km` : '—'}
          </span>
        </div>

        {/* Floating Live Telemetry Badge 2: Traktionsbatterie SoC (Bottom Left) */}
        <div style={{
          position: 'absolute',
          bottom: '8%',
          left: '2%',
          background: 'rgba(255, 255, 255, 0.95)',
          backdropFilter: 'blur(8px)',
          border: `2px solid ${socColor}`,
          borderRadius: 12,
          padding: '8px 14px',
          boxShadow: '0 6px 16px rgba(0, 0, 0, 0.12)',
          display: 'flex',
          flexDirection: 'column',
        }}>
          <span style={{ fontSize: 10, fontWeight: 700, color: '#64748b', textTransform: 'uppercase' }}>
            🔋 Traktionsbatterie
          </span>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 6 }}>
            <span style={{ fontSize: 24, fontWeight: 900, color: socColor, lineHeight: 1.1 }}>
              {batteryPct.toFixed(0)}%
            </span>
            <span style={{ fontSize: 11, color: '#64748b', fontWeight: 600 }}>
              (Ziel: {data?.targetSoCPct ? `${data.targetSoCPct}%` : '80%'})
            </span>
          </div>
          {/* Mini SoC progress bar */}
          <div style={{ width: 120, height: 6, background: '#e2e8f0', borderRadius: 999, overflow: 'hidden', marginTop: 4 }}>
            <div style={{ width: `${Math.min(100, Math.max(0, batteryPct))}%`, height: '100%', background: socColor }} />
          </div>
        </div>

        {/* Floating Live Telemetry Badge 3: Ladeleistung / Status (Top Right / Rear) */}
        <div style={{
          position: 'absolute',
          top: '8%',
          right: '2%',
          background: 'rgba(255, 255, 255, 0.95)',
          backdropFilter: 'blur(8px)',
          border: `1.5px solid ${isCharging ? '#16a34a' : isConnected ? '#4f46e5' : '#cbd5e1'}`,
          borderRadius: 12,
          padding: '8px 14px',
          boxShadow: '0 6px 16px rgba(0, 0, 0, 0.12)',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'flex-start',
        }}>
          <span style={{ fontSize: 10, fontWeight: 700, color: '#64748b', textTransform: 'uppercase' }}>
            {isCharging ? '⚡ Ladeleistung' : '🔌 Ladeanschluss'}
          </span>
          <span style={{ fontSize: 20, fontWeight: 800, color: isCharging ? '#15803d' : isConnected ? '#3730a3' : '#334155', lineHeight: 1.1 }}>
            {isCharging ? `${chargePower} kW` : isConnected ? 'Verbunden' : 'Getrennt'}
          </span>
          {isCharging && data?.remainingChargingTimeMin ? (
            <span style={{ fontSize: 11, color: '#16a34a', fontWeight: 600, marginTop: 2 }}>
              ⏳ noch ca. {data.remainingChargingTimeMin} min
            </span>
          ) : null}
        </div>

        {/* Floating Live Telemetry Badge 4: Innenraum-Klima & Schlösser (Bottom Right) */}
        <div style={{
          position: 'absolute',
          bottom: '8%',
          right: '2%',
          background: 'rgba(255, 255, 255, 0.95)',
          backdropFilter: 'blur(8px)',
          border: '1.5px solid #0f766e',
          borderRadius: 12,
          padding: '8px 14px',
          boxShadow: '0 6px 16px rgba(15, 118, 110, 0.14)',
          display: 'flex',
          flexDirection: 'column',
        }}>
          <span style={{ fontSize: 10, fontWeight: 700, color: '#64748b', textTransform: 'uppercase' }}>
            🌡️ Innenraum / Klima
          </span>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 6 }}>
            <span style={{ fontSize: 18, fontWeight: 800, color: '#0f766e', lineHeight: 1.1 }}>
              {targetTemp} °C
            </span>
            {outdoorTemp !== null && (
              <span style={{ fontSize: 11, color: '#64748b' }}>
                (Außen {outdoorTemp}°)
              </span>
            )}
          </div>
          <div style={{ fontSize: 11, fontWeight: 700, color: isLocked ? '#334155' : '#dc2626', marginTop: 3 }}>
            {isLocked ? '🔒 Zentralverriegelt' : '🔓 Fahrzeug offen'}
          </div>
        </div>

        {/* Center Bottom Mileage Badge */}
        {mileage && (
          <div style={{
            position: 'absolute',
            bottom: '2%',
            background: 'rgba(15, 23, 42, 0.85)',
            color: '#ffffff',
            backdropFilter: 'blur(6px)',
            borderRadius: 20,
            padding: '4px 14px',
            fontSize: 12,
            fontWeight: 700,
            boxShadow: '0 4px 10px rgba(0, 0, 0, 0.2)',
          }}>
            📍 Gesamtlaufleistung: {mileage} km
          </div>
        )}
      </div>

      {/* Helper Subtext */}
      <div style={{ textAlign: 'center', marginTop: 10, fontSize: 12, color: '#64748b' }}>
        📸 <em>Originalgetreues 1:43 Metall-Modell des Škoda Enyaq Coupé RS in Race-Blau Metallic mit direkt überlagerten Echtzeit-Fahrzeugdaten.</em>
      </div>
    </div>
  )
}
