import { useEffect, useState } from 'react'

const API = 'http://192.168.188.97:8083/api/wallbox'

type WallboxStatus = {
  amp?: number
  frc?: number
  car?: number
}

type Feedback = {
  type: 'success' | 'error'
  msg: string
}

type SetResponse = {
  ok?: boolean
  error?: string
}

export default function GoE() {
  const [status, setStatus] = useState<WallboxStatus | null>(null)
  const [sending, setSending] = useState(false)
  const [feedback, setFeedback] = useState<Feedback | null>(null)
  const [timeFrom, setTimeFrom] = useState('00:00')
  const [timeTo, setTimeTo] = useState('00:00')

  const loadStatus = async () => {
    try {
      const r = await fetch(`${API}/status`)
      if (r.ok) setStatus(await r.json())
    } catch { /* ignore */ }
  }

  useEffect(() => {
    loadStatus()
    const t = setInterval(loadStatus, 5000)
    return () => clearInterval(t)
  }, [])

  const sendSet = async (key: string, value: unknown) => {
    setSending(true)
    setFeedback(null)
    try {
      const r = await fetch(`${API}/set`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key, value }),
      })
      const data: SetResponse = await r.json()
      if (data.ok) {
        setFeedback({ type: 'success', msg: `✅ ${key} = ${String(value)} gesendet` })
      } else {
        setFeedback({ type: 'error', msg: `❌ ${data.error}` })
      }
      setTimeout(loadStatus, 1000)
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      setFeedback({ type: 'error', msg: `❌ Fehler: ${message}` })
    } finally {
      setSending(false)
    }
  }

  const frcLabels: Record<number, string> = { 0: 'Neutral', 1: 'Aus (Idle)', 2: 'Laden erzwingen' }
  const carLabels: Record<number, string> = { 1: 'Kein Fahrzeug', 2: 'Laden', 3: 'Warten', 4: 'Fertig' }

  const cardStyle = {
    background: '#fff',
    borderRadius: 10,
    padding: '20px 24px',
    boxShadow: '0 1px 4px rgba(0,0,0,0.08)',
  }

  const btnStyle = (color = '#3b82f6') => ({
    padding: '10px 20px',
    borderRadius: 8,
    border: 'none',
    background: color,
    color: '#fff',
    fontWeight: 600,
    fontSize: 14,
    cursor: sending ? 'wait' : 'pointer',
    opacity: sending ? 0.6 : 1,
    transition: 'opacity 0.2s',
  })

  return (
    <div style={{ maxWidth: 700, margin: '0 auto' }}>
      <h1 style={{ marginBottom: 4 }}>Wallbox Steuerung</h1>
      <p style={{ color: '#6b7280', marginTop: 0, marginBottom: 24 }}>
        go-eCharger Einstellungen per MQTT setzen
      </p>

      {feedback && (
        <div style={{
          padding: '10px 16px',
          borderRadius: 8,
          marginBottom: 16,
          background: feedback.type === 'success' ? '#d1fae5' : '#fee2e2',
          color: feedback.type === 'success' ? '#065f46' : '#991b1b',
          fontWeight: 500,
        }}>
          {feedback.msg}
        </div>
      )}

      <div style={{ ...cardStyle, marginBottom: 16 }}>
        <h3 style={{ marginTop: 0, color: '#374151' }}>📊 Aktueller Status</h3>
        {status ? (
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 12 }}>
            <StatusItem label="Ladestrom" value={`${status.amp ?? '—'} A`} />
            <StatusItem label="Modus" value={status.frc === undefined ? '—' : frcLabels[status.frc] ?? status.frc} />
            <StatusItem label="Auto" value={status.car === undefined ? '—' : carLabels[status.car] ?? status.car} />
          </div>
        ) : (
          <p style={{ color: '#9ca3af' }}>Lade Status…</p>
        )}
      </div>

      <div style={{ ...cardStyle, marginBottom: 16 }}>
        <h3 style={{ marginTop: 0, color: '#374151' }}>⚡ Ladestrom (amp)</h3>
        <p style={{ color: '#6b7280', fontSize: 14, margin: '0 0 12px' }}>Strom in Ampere setzen (6–16 A)</p>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          {[6, 8, 10, 12, 14, 16].map(a => (
            <button key={a} style={btnStyle(status?.amp === a ? '#1d4ed8' : '#3b82f6')} disabled={sending} onClick={() => sendSet('amp', a)}>{a} A</button>
          ))}
        </div>
      </div>

      <div style={{ ...cardStyle, marginBottom: 16 }}>
        <h3 style={{ marginTop: 0, color: '#374151' }}>🔌 Lademodus (frc)</h3>
        <p style={{ color: '#6b7280', fontSize: 14, margin: '0 0 12px' }}>Force-State der Wallbox steuern</p>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <button style={btnStyle('#6b7280')} disabled={sending} onClick={() => sendSet('frc', 0)}>Neutral (0)</button>
          <button style={btnStyle('#ef4444')} disabled={sending} onClick={() => sendSet('frc', 1)}>Aus / Idle (1)</button>
          <button style={btnStyle('#10b981')} disabled={sending} onClick={() => sendSet('frc', 2)}>Laden erzwingen (2)</button>
        </div>
      </div>

      <div style={{ ...cardStyle, marginBottom: 16 }}>
        <h3 style={{ marginTop: 0, color: '#374151' }}>🔄 Phasenumschaltung (psm)</h3>
        <p style={{ color: '#6b7280', fontSize: 14, margin: '0 0 12px' }}>1-phasig oder 3-phasig laden</p>
        <div style={{ display: 'flex', gap: 8 }}>
          <button style={btnStyle('#8b5cf6')} disabled={sending} onClick={() => sendSet('psm', 1)}>1-phasig</button>
          <button style={btnStyle('#8b5cf6')} disabled={sending} onClick={() => sendSet('psm', 2)}>3-phasig</button>
        </div>
      </div>

      <div style={{ ...cardStyle, marginBottom: 16 }}>
        <h3 style={{ marginTop: 0, color: '#374151' }}>⏱️ Zeitfenster setzen</h3>
        <p style={{ color: '#6b7280', fontSize: 14, margin: '0 0 12px' }}>Zwei Uhrzeiten (HH:MM), unabhängig vom Datum</p>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
          <label style={{ display: 'flex', flexDirection: 'column', fontSize: 12, color: '#6b7280' }}>Von
            <input type="time" value={timeFrom} onChange={e => setTimeFrom(e.target.value)} style={{ marginTop: 6, padding: '6px 8px', borderRadius: 6, border: '1px solid #e5e7eb' }} />
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', fontSize: 12, color: '#6b7280' }}>Bis
            <input type="time" value={timeTo} onChange={e => setTimeTo(e.target.value)} style={{ marginTop: 6, padding: '6px 8px', borderRadius: 6, border: '1px solid #e5e7eb' }} />
          </label>
          <button style={btnStyle('#f59e0b')} disabled={sending} onClick={() => sendSet('ato', { from: timeFrom, to: timeTo })}>Zeitfenster senden</button>
        </div>
      </div>
    </div>
  )
}

type StatusItemProps = {
  label: string
  value: string | number
}

function StatusItem({ label, value }: StatusItemProps) {
  return (
    <div style={{ background: '#f9fafb', borderRadius: 8, padding: '10px 14px', textAlign: 'center' }}>
      <div style={{ fontSize: 12, color: '#6b7280', marginBottom: 4 }}>{label}</div>
      <div style={{ fontSize: 18, fontWeight: 700, color: '#111827' }}>{value}</div>
    </div>
  )
}