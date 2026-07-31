import React, { useState, useEffect } from 'react'

const API = '/api/huehnerklappe'

export default function Huehnerklappe() {
    const [sleepTime, setSleepTime] = useState(60) // default 60 Sekunden
  const [sleepUntil, setSleepUntil] = useState('')
  const [scheduleTimestamps, setScheduleTimestamps] = useState(['06:30:00', '12:00:00', '18:30:00'])
  const [awakeSeconds, setAwakeSeconds] = useState(30)
  const [pickerIndex, setPickerIndex] = useState(null)
  const [pickerDraft, setPickerDraft] = useState({ hour: '00', minute: '00', second: '00' })
  const [status, setStatus] = useState(null)
  const [sending, setSending] = useState(false)
  const [feedback, setFeedback] = useState(null)
  const [battery, setBattery] = useState(null)
  const [wakeReason, setWakeReason] = useState(null)
  const [charging, setCharging] = useState(null)
  const [clockOffsetMs, setClockOffsetMs] = useState(0)

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
        if (Number.isFinite(data.serverNowMs)) {
          setClockOffsetMs(Date.now() - data.serverNowMs)
        }
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

  const updateScheduleTimestamp = (index, value) => {
    setScheduleTimestamps(prev => {
      const next = [...prev]
      next[index] = value
      return next
    })
  }

  const addScheduleTimestamp = () => {
    setScheduleTimestamps(prev => {
      if (prev.length >= 20) {
        return prev
      }
      return [...prev, '00:00:00']
    })
  }

  const removeScheduleTimestamp = (index) => {
    setScheduleTimestamps(prev => {
      if (prev.length <= 1) {
        return prev
      }
      return prev.filter((_, i) => i !== index)
    })
  }

  const pad2 = (value) => String(value).padStart(2, '0')

  const parseTimestampToDraft = (timestamp) => {
    const [hour = '00', minute = '00', second = '00'] = String(timestamp || '').split(':')
    return {
      hour: pad2(Number(hour) || 0),
      minute: pad2(Number(minute) || 0),
      second: pad2(Number(second) || 0),
    }
  }

  const draftToTimestamp = (draft) => `${draft.hour}:${draft.minute}:${draft.second}`

  const openTimestampPicker = (index) => {
    setPickerDraft(parseTimestampToDraft(scheduleTimestamps[index]))
    setPickerIndex(index)
  }

  const applyTimestampPicker = () => {
    if (pickerIndex === null) {
      return
    }
    const value = draftToTimestamp(pickerDraft)
    updateScheduleTimestamp(pickerIndex, value)
    setPickerIndex(null)
  }

  const sendSleepSchedule = async () => {
    const timestamps = scheduleTimestamps.map(v => v.trim())
    if (timestamps.some(v => !v)) {
      setFeedback({ type: 'error', msg: '❌ Bitte alle Timestamp-Felder ausfüllen.' })
      return
    }

    setSending(true)
    setFeedback(null)
    try {
      const r = await fetch(`${API}/sleep-schedule`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          count: timestamps.length,
          timestamps,
          awakeSeconds,
        }),
      })
      const data = await r.json()
      if (data.ok) {
        if (data.stored) {
          setFeedback({ type: 'success', msg: `✅ ${timestamps.length} Timestamps gespeichert (wach: ${awakeSeconds}s)` })
        } else {
          setFeedback({ type: 'success', msg: `✅ ${timestamps.length} Timestamps gesendet (wach: ${awakeSeconds}s)` })
        }
      } else {
        setFeedback({ type: 'error', msg: `❌ ${data.error}` })
      }
    } catch (err) {
      setFeedback({ type: 'error', msg: `❌ Fehler: ${err.message}` })
    } finally {
      setSending(false)
    }
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

  const ghostBtnStyle = {
    padding: '8px 14px',
    borderRadius: 10,
    border: '1px solid #c7d2fe',
    background: '#eef2ff',
    color: '#3730a3',
    fontWeight: 700,
    fontSize: 13,
    cursor: sending ? 'wait' : 'pointer',
    opacity: sending ? 0.6 : 1,
  }

  const removeBtnStyle = {
    padding: '8px 12px',
    borderRadius: 10,
    border: '1px solid #fecaca',
    background: '#fff1f2',
    color: '#be123c',
    fontWeight: 800,
    fontSize: 13,
    cursor: sending ? 'wait' : 'pointer',
    opacity: sending ? 0.6 : 1,
    minWidth: 34,
  }

  const selectedTheme = {
    card: {
      marginTop: 20,
      borderTop: '1px solid #e5e7eb',
      background: '#ffffff',
      borderRadius: 12,
      border: '1px solid #e5e7eb',
      padding: 16,
    },
    titleColor: '#1f2937',
    subtitleColor: '#6b7280',
    labelColor: '#374151',
    rowBg: '#f9fafb',
    rowBorder: '#e5e7eb',
    inputBg: '#ffffff',
    inputBorder: '#d1d5db',
    addBg: '#f3f4f6',
    addBorder: '#d1d5db',
    addColor: '#374151',
    removeBg: '#fff1f2',
    removeBorder: '#fecaca',
    removeColor: '#be123c',
    sendColor: '#047857',
  }

  const formatStatusTimestamp = (unixMs) => {
    if (Number.isFinite(unixMs) && unixMs > 0) {
      const correctedMs = unixMs + clockOffsetMs
      return new Intl.DateTimeFormat('de-DE', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false,
      }).format(new Date(correctedMs))
    }
    return '—'
  }

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
            <StatusItem label="Sleep gesendet" value={formatStatusTimestamp(status.sleepCommandAtMs)} />
            <StatusItem label="Sleeping seit" value={formatStatusTimestamp(status.sleepingAtMs)} />
            <StatusItem label="Online seit" value={formatStatusTimestamp(status.onlineAtMs)} />
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

        <div style={selectedTheme.card}>
          <h4 style={{ marginTop: 0, marginBottom: 8, color: selectedTheme.titleColor, fontSize: 18 }}>Sleep-Schedule per Timestamps</h4>
          <p style={{ marginTop: 0, marginBottom: 14, color: selectedTheme.subtitleColor, fontSize: 13 }}>
            Zeiten werden in Reihenfolge gespeichert und nacheinander abgearbeitet.
          </p>
          <div style={{ display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap', marginBottom: 12 }}>
            <div style={{ fontSize: 14, color: selectedTheme.labelColor, display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ fontWeight: 700 }}>Anzahl Timestamps: {scheduleTimestamps.length}</span>
              <button
                type="button"
                onClick={addScheduleTimestamp}
                disabled={sending || scheduleTimestamps.length >= 20}
                style={{
                  ...ghostBtnStyle,
                  background: selectedTheme.addBg,
                  border: `1px solid ${selectedTheme.addBorder}`,
                  color: selectedTheme.addColor,
                }}
              >
                + Hinzufuegen
              </button>
            </div>
            <label style={{ fontSize: 14, color: selectedTheme.labelColor, fontWeight: 600 }}>
              Wachzeit bis nächster Sleep (Sekunden):
              <input
                type="number"
                min={0}
                max={86400}
                value={awakeSeconds}
                onChange={e => setAwakeSeconds(Math.max(0, Number(e.target.value) || 0))}
                style={{ marginLeft: 8, padding: '8px 10px', borderRadius: 10, border: `1px solid ${selectedTheme.inputBorder}`, width: 100, background: selectedTheme.inputBg, color: selectedTheme.labelColor }}
              />
            </label>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            {scheduleTimestamps.map((timestamp, index) => (
              <div key={index} style={{
                fontSize: 14,
                color: selectedTheme.labelColor,
                display: 'flex',
                alignItems: 'flex-start',
                gap: 10,
                background: selectedTheme.rowBg,
                border: `1px solid ${selectedTheme.rowBorder}`,
                borderRadius: 12,
                padding: '10px 12px',
                position: 'relative',
              }}>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 8, flex: 1 }}>
                  <label style={{ display: 'flex', alignItems: 'center', gap: 8, fontWeight: 600, flexWrap: 'wrap' }}>
                    <span style={{ minWidth: 92 }}>Timestamp {index + 1}:</span>
                    <button
                      type="button"
                      onClick={() => openTimestampPicker(index)}
                      style={{
                        padding: '8px 10px',
                        borderRadius: 10,
                        border: `1px solid ${selectedTheme.inputBorder}`,
                        background: selectedTheme.inputBg,
                        color: selectedTheme.labelColor,
                        minWidth: 120,
                        textAlign: 'left',
                        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, Liberation Mono, Courier New, monospace',
                      }}
                    >
                      {timestamp}
                    </button>
                  </label>

                  {pickerIndex === index && (
                    <div style={{
                      border: `1px solid ${selectedTheme.inputBorder}`,
                      borderRadius: 10,
                      background: selectedTheme.rowBg,
                      padding: 10,
                      display: 'flex',
                      flexDirection: 'column',
                      gap: 10,
                      boxShadow: '0 8px 22px rgba(2, 6, 23, 0.15)',
                      maxWidth: 360,
                    }}>
                      <div style={{ fontWeight: 700, fontSize: 12, opacity: 0.85 }}>Zeit auswählen</div>
                      <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                        <select
                          value={pickerDraft.hour}
                          onChange={(e) => setPickerDraft(prev => ({ ...prev, hour: e.target.value }))}
                          style={{ padding: '7px 8px', borderRadius: 8, border: `1px solid ${selectedTheme.inputBorder}`, background: selectedTheme.inputBg, color: selectedTheme.labelColor }}
                        >
                          {Array.from({ length: 24 }, (_, i) => pad2(i)).map((h) => (
                            <option key={h} value={h}>{h}</option>
                          ))}
                        </select>
                        <span>:</span>
                        <select
                          value={pickerDraft.minute}
                          onChange={(e) => setPickerDraft(prev => ({ ...prev, minute: e.target.value }))}
                          style={{ padding: '7px 8px', borderRadius: 8, border: `1px solid ${selectedTheme.inputBorder}`, background: selectedTheme.inputBg, color: selectedTheme.labelColor }}
                        >
                          {Array.from({ length: 60 }, (_, i) => pad2(i)).map((m) => (
                            <option key={m} value={m}>{m}</option>
                          ))}
                        </select>
                        <span>:</span>
                        <select
                          value={pickerDraft.second}
                          onChange={(e) => setPickerDraft(prev => ({ ...prev, second: e.target.value }))}
                          style={{ padding: '7px 8px', borderRadius: 8, border: `1px solid ${selectedTheme.inputBorder}`, background: selectedTheme.inputBg, color: selectedTheme.labelColor }}
                        >
                          {Array.from({ length: 60 }, (_, i) => pad2(i)).map((s) => (
                            <option key={s} value={s}>{s}</option>
                          ))}
                        </select>
                      </div>
                      <div style={{ display: 'flex', gap: 8 }}>
                        <button
                          type="button"
                          onClick={applyTimestampPicker}
                          style={{ ...ghostBtnStyle, background: selectedTheme.addBg, border: `1px solid ${selectedTheme.addBorder}`, color: selectedTheme.addColor }}
                        >
                          Uebernehmen
                        </button>
                        <button
                          type="button"
                          onClick={() => setPickerIndex(null)}
                          style={{ ...removeBtnStyle, background: '#f4f4f5', border: '1px solid #d4d4d8', color: '#3f3f46' }}
                        >
                          Abbrechen
                        </button>
                      </div>
                    </div>
                  )}
                </div>
                <button
                  type="button"
                  onClick={() => removeScheduleTimestamp(index)}
                  disabled={sending || scheduleTimestamps.length <= 1}
                  style={{
                    ...removeBtnStyle,
                    background: selectedTheme.removeBg,
                    border: `1px solid ${selectedTheme.removeBorder}`,
                    color: selectedTheme.removeColor,
                  }}
                >
                  Entfernen
                </button>
              </div>
            ))}
          </div>
          <div style={{ marginTop: 12 }}>
            <button
              style={btnStyle(selectedTheme.sendColor)}
              disabled={sending}
              onClick={sendSleepSchedule}
            >
              Timestamp-Schedule senden
            </button>
          </div>
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
