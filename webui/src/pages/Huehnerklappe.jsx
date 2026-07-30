import React, { useState, useEffect } from 'react'

const API = '/api/huehnerklappe'

export default function Huehnerklappe() {
    const [sleepTime, setSleepTime] = useState(60) // default 60 Sekunden
  const [sleepUntil, setSleepUntil] = useState('')
  const [status, setStatus] = useState(null)
  const [sending, setSending] = useState(false)
  const [feedback, setFeedback] = useState(null)
  const [battery, setBattery] = useState(null)
  const [wakeReason, setWakeReason] = useState(null)
  const [charging, setCharging] = useState(null)

  // Status laden
  const loadStatus = async () => {
    try {
      const r = await fetch(`${API}/status`)
      if (r.ok) {
        const data = await r.json()
        setStatus(data)
        setBattery(data.battery ?? null)
        setWakeReason(data.wakeReason ?? null)
        setCharging(data.charging ?? null)
      }
    } catch { /* ignore */ }
  }

  useEffect(() => {
    loadStatus()
    const t = setInterval(loadStatus, 5000)
    return () => clearInterval(t)
  }, [])

  // Befehl senden
  const sendCommand = async (key, value = null, successMessage = null) => {
    setSending(true)
    setFeedback(null)
    try {
      const r = await fetch(`${API}/set`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key, value }),
      })
      const data = await r.json()
      if (data.ok) {
        setFeedback({ type: 'success', msg: successMessage ?? `✅ ${key} gesendet` })
      } else {
        setFeedback({ type: 'error', msg: `❌ ${data.error}` })
      }
      setTimeout(loadStatus, 1000)
    } catch (err) {
      setFeedback({ type: 'error', msg: `❌ Fehler: ${err.message}` })
    } finally {
      setSending(false)
    }
  }

  const sleepSecondsUntil = (targetTime) => {
    const parts = targetTime.split(':').map(Number)
    if (parts.length < 2 || parts.some(Number.isNaN)) {
      return null
    }

    const now = new Date()
    const target = new Date(now)
    target.setHours(parts[0], parts[1], parts[2] ?? 0, 0)

    if (target <= now) {
      target.setDate(target.getDate() + 1)
    }

    const diffSeconds = Math.ceil((target.getTime() - now.getTime()) / 1000)
    return Math.max(1, diffSeconds)
  }

  const sendSleepUntil = async () => {
    if (!sleepUntil) {
      setFeedback({ type: 'error', msg: '❌ Bitte zuerst eine Uhrzeit auswählen.' })
      return
    }

    const seconds = sleepSecondsUntil(sleepUntil)
    if (!seconds) {
      setFeedback({ type: 'error', msg: '❌ Ungültige Uhrzeit.' })
      return
    }

    await sendCommand('engine/sleep', seconds, `✅ Sleep bis ${sleepUntil} gesendet (${seconds}s)`)
  }

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
      <h1 style={{ marginBottom: 4 }}>Hühnerklappe Steuerung</h1>
      <p style={{ color: '#6b7280', marginTop: 0, marginBottom: 24 }}>
        Klappe öffnen/schließen und Status anzeigen
      </p>

      {/* Feedback */}
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

      {/* Status */}
      <div style={{ ...cardStyle, marginBottom: 16 }}>
        <h3 style={{ marginTop: 0, color: '#374151' }}>📊 Aktueller Status</h3>
        {status ? (
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 12 }}>
            <StatusItem label="Akku" value={battery !== null ? `${battery}%` : '—'} />
            <StatusItem label="Charging" value={charging ?? '—'} />
            <StatusItem label="IP" value={status.ip ?? '—'} />
            <StatusItem label="Position" value={status.position} />
            <StatusItem label="Letzte Aktion" value={status.lastAction} />
            <StatusItem label="Controller" value={status.controllerState ?? '—'} />
            <StatusItem label="Sleep-Status" value={status.sleepState ?? '—'} />
            <StatusItem label="Weckgrund" value={wakeReason ?? '—'} />
            <StatusItem label="Sleep gesendet" value={status.sleepCommandAt ?? '—'} />
            <StatusItem label="Sleeping seit" value={status.sleepingAt ?? '—'} />
            <StatusItem label="Online seit" value={status.onlineAt ?? '—'} />
            <StatusItem label="Diff sleeping→online" value={status.wakeDeltaMs !== undefined && status.wakeDeltaMs !== null ? `${status.wakeDeltaMs} ms` : '—'} />
            <StatusItem label="Fehler" value={status.error ?? '—'} />
          </div>
        ) : (
          <p style={{ color: '#9ca3af' }}>Lade Status…</p>
        )}
      </div>

      {/* Steuerung */}
      <div style={{ ...cardStyle }}>
        <h3 style={{ marginTop: 0, color: '#374151' }}>🔧 Klappe steuern</h3>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
          <button style={btnStyle('#10b981')} disabled={sending} onClick={() => sendCommand('engine','open')}>
            Öffnen
          </button>
          <button style={btnStyle('#ef4444')} disabled={sending} onClick={() => sendCommand('engine','close')}>
            Schließen
          </button>
          <button style={btnStyle('#f59e0b')} disabled={sending} onClick={() => sendCommand('engine','stop')}>
            Stop
          </button>
        </div>
        <div style={{ marginTop: 24, display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
          <label style={{ fontSize: 14, color: '#6b7280' }}>
            Sleep-Dauer (Sekunden):
            <input
              type="number"
              min={1}
              max={86400}
              value={sleepTime}
              onChange={e => setSleepTime(Number(e.target.value))}
              style={{ marginLeft: 8, padding: '6px 8px', borderRadius: 6, border: '1px solid #e5e7eb', width: 80 }}
            />
          </label>
          <button
            style={btnStyle('#6366f1')}
            disabled={sending}
            onClick={() => sendCommand('engine/sleep', sleepTime )}
          >
            Controller schlafen lassen
          </button>
        </div>
        <div style={{ marginTop: 14, display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
          <label style={{ fontSize: 14, color: '#6b7280' }}>
            Schlafen bis (Uhrzeit):
            <input
              type="time"
              step={1}
              value={sleepUntil}
              onChange={e => setSleepUntil(e.target.value)}
              style={{ marginLeft: 8, padding: '6px 8px', borderRadius: 6, border: '1px solid #e5e7eb' }}
            />
          </label>
          <button
            style={btnStyle('#4338ca')}
            disabled={sending}
            onClick={sendSleepUntil}
          >
            Bis Uhrzeit schlafen lassen
          </button>
        </div>
      </div>
    </div>
  )
}

function StatusItem({ label, value }) {
  return (
    <div style={{
      background: '#f9fafb',
      borderRadius: 8,
      padding: '10px 14px',
      textAlign: 'center',
    }}>
      <div style={{ fontSize: 12, color: '#6b7280', marginBottom: 4 }}>{label}</div>
      <div style={{ fontSize: 18, fontWeight: 700, color: '#111827' }}>{value}</div>
    </div>
  )
}
