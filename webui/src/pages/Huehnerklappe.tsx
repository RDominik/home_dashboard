import type { CSSProperties } from 'react'
import { useEffect, useState } from 'react'

const API = '/api/huehnerklappe'

type ControlMode = 'manual' | 'schedule'
type ScheduleAction = 'open' | 'close' | 'stop' | 'none'

type Feedback = {
  type: 'success' | 'error'
  msg: string
}

type ScheduleHistoryEntry = {
  sleepSeconds?: number
  batteryPercent?: string
  sleepCommandAtMs?: number
  sleepingAtMs?: number
  wokeUpAtMs?: number
}

type HuehnerklappeStatus = {
  position?: string
  lastAction?: string
  error?: string
  battery?: string
  wakeReason?: string
  controllerState?: string
  sleepState?: string
  ip?: string
  charging?: string
  scheduleActive?: boolean
  scheduleTimezone?: string
  serverNowMs?: number
  sleepCommandAtMs?: number
  sleepingAtMs?: number
  onlineAtMs?: number
  wakeDeltaMs?: number
  scheduleHistory?: ScheduleHistoryEntry[]
}

type UiStateResponse = {
  sleepTime?: number
  motorAutoStopSeconds?: number
  sleepUntil?: string
  controlMode?: ControlMode
  scheduleActive?: boolean
  scheduleTimestamps?: string[]
  scheduleEntries?: Array<{ timestamp?: string; action?: ScheduleAction }>
  awakeSeconds?: number
  historyExpanded?: boolean
}

type SetCommandResponse = {
  ok?: boolean
  error?: string
  stored?: boolean
}

type PickerDraft = {
  hour: string
  minute: string
  second: string
}

export default function Huehnerklappe() {
  const [sleepTime, setSleepTime] = useState(60) // default 60 Sekunden
  const [motorAutoStopSeconds, setMotorAutoStopSeconds] = useState(15)
  const [sleepUntil, setSleepUntil] = useState('')
  const [controlMode, setControlMode] = useState<ControlMode>('manual')
  const [scheduleActive, setScheduleActive] = useState(false)
  const [scheduleTimestamps, setScheduleTimestamps] = useState(['06:30:00', '12:00:00', '18:30:00'])
  const [scheduleActions, setScheduleActions] = useState<ScheduleAction[]>(['none', 'none', 'none'])
  const [awakeSeconds, setAwakeSeconds] = useState(30)
  const [pickerIndex, setPickerIndex] = useState<number | null>(null)
  const [pickerDraft, setPickerDraft] = useState<PickerDraft>({ hour: '00', minute: '00', second: '00' })
  const [historyExpanded, setHistoryExpanded] = useState(false)
  const [status, setStatus] = useState<HuehnerklappeStatus | null>(null)
  const [sending, setSending] = useState(false)
  const [feedback, setFeedback] = useState<Feedback | null>(null)
  const [battery, setBattery] = useState<string | null>(null)
  const [wakeReason, setWakeReason] = useState<string | null>(null)
  const [charging, setCharging] = useState<string | null>(null)
  const [clockOffsetMs, setClockOffsetMs] = useState(0)
  const [uiLoaded, setUiLoaded] = useState(false)

  // Status laden
  const loadStatus = async () => {
    try {
      const r = await fetch(`${API}/status`)
      if (r.ok) {
        const data: HuehnerklappeStatus = await r.json()
        setStatus(data)
        setBattery(data.battery ?? null)
        setWakeReason(data.wakeReason ?? null)
        setCharging(data.charging ?? null)
        if (typeof data.serverNowMs === 'number' && Number.isFinite(data.serverNowMs)) {
          setClockOffsetMs(Date.now() - data.serverNowMs)
        }
        if (typeof data.scheduleActive === 'boolean') {
          setScheduleActive(data.scheduleActive)
        }
      }
    } catch { /* ignore */ }
  }

  const loadUiState = async () => {
    try {
      const r = await fetch(`${API}/ui-state`)
      if (!r.ok) {
        return
      }

      const data: UiStateResponse = await r.json()

      if (Number.isFinite(data.sleepTime)) {
        setSleepTime(Math.max(1, Math.min(86400, Number(data.sleepTime))))
      }
      if (Number.isFinite(data.motorAutoStopSeconds)) {
        setMotorAutoStopSeconds(Math.max(1, Math.min(60, Number(data.motorAutoStopSeconds))))
      }
      if (typeof data.sleepUntil === 'string') {
        setSleepUntil(data.sleepUntil)
      }
      if (data.controlMode === 'manual' || data.controlMode === 'schedule') {
        setControlMode(data.controlMode)
      } else if (data.scheduleActive) {
        setControlMode('schedule')
      } else {
        setControlMode('manual')
      }
      if (Array.isArray(data.scheduleTimestamps) && data.scheduleTimestamps.length > 0) {
        const cleaned = data.scheduleTimestamps
          .map((v) => String(v).trim())
          .filter(Boolean)
          .slice(0, 20)
        if (cleaned.length > 0) {
          setScheduleTimestamps(cleaned)
          setScheduleActions(cleaned.map(() => 'none'))
        }
      }
      if (Array.isArray(data.scheduleEntries) && data.scheduleEntries.length > 0) {
        const entries = data.scheduleEntries
          .map((entry) => ({
            timestamp: String(entry.timestamp ?? '').trim(),
            action: entry.action === 'open' || entry.action === 'close' || entry.action === 'stop' || entry.action === 'none'
              ? entry.action
              : 'none' as ScheduleAction,
          }))
          .filter((entry) => entry.timestamp)
          .slice(0, 20)
        if (entries.length > 0) {
          setScheduleTimestamps(entries.map((entry) => entry.timestamp))
          setScheduleActions(entries.map((entry) => entry.action))
        }
      }
      if (Number.isFinite(data.awakeSeconds)) {
        setAwakeSeconds(Math.max(0, Math.min(86400, Number(data.awakeSeconds))))
      }
      if (typeof data.historyExpanded === 'boolean') {
        setHistoryExpanded(data.historyExpanded)
      }
    } catch {
      // Ignore transient load errors.
    }
  }

  useEffect(() => {
    let cancelled = false

    const init = async () => {
      await Promise.all([loadStatus(), loadUiState()])
      if (!cancelled) {
        setUiLoaded(true)
      }
    }

    init()
    const t = setInterval(loadStatus, 5000)
    return () => {
      cancelled = true
      clearInterval(t)
    }
  }, [])

  useEffect(() => {
    if (!uiLoaded) {
      return
    }

    const payload = {
      sleepTime,
      motorAutoStopSeconds,
      sleepUntil,
      controlMode,
      scheduleActive,
      scheduleTimestamps,
      scheduleEntries: scheduleTimestamps.map((timestamp, index) => ({
        timestamp,
        action: scheduleActions[index] ?? 'none',
      })),
      awakeSeconds,
      historyExpanded,
    }

    const timer = setTimeout(() => {
      fetch(`${API}/ui-state`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      }).catch(() => {})
    }, 250)

    return () => clearTimeout(timer)
  }, [uiLoaded, sleepTime, motorAutoStopSeconds, sleepUntil, controlMode, scheduleActive, scheduleTimestamps, scheduleActions, awakeSeconds, historyExpanded])

  const sendCommand = async (key: string, value: string | number | null = null, successMessage: string | null = null) => {
    setSending(true)
    setFeedback(null)
    try {
      const r = await fetch(`${API}/set`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key, value }),
      })
      const data: SetCommandResponse = await r.json()
      if (data.ok) {
        setFeedback({ type: 'success', msg: successMessage ?? `✅ ${key} gesendet` })
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

  const sleepSecondsUntil = (targetTime: string): number | null => {
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

  const updateScheduleTimestamp = (index: number, value: string) => {
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
    setScheduleActions(prev => [...prev, 'none'])
  }

  const removeScheduleTimestamp = (index: number) => {
    setScheduleTimestamps(prev => {
      if (prev.length <= 1) {
        return prev
      }
      return prev.filter((_, i) => i !== index)
    })
    setScheduleActions(prev => prev.filter((_, i) => i !== index))
  }

  const updateScheduleAction = (index: number, action: ScheduleAction) => {
    setScheduleActions(prev => {
      const next = [...prev]
      next[index] = action
      return next
    })
  }

  const pad2 = (value: number) => String(value).padStart(2, '0')

  const parseTimestampToDraft = (timestamp: string) => {
    const [hour = '00', minute = '00', second = '00'] = String(timestamp || '').split(':')
    return {
      hour: pad2(Number(hour) || 0),
      minute: pad2(Number(minute) || 0),
      second: pad2(Number(second) || 0),
    }
  }

  const draftToTimestamp = (draft: PickerDraft) => `${draft.hour}:${draft.minute}:${draft.second}`

  const openTimestampPicker = (index: number) => {
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

  const sendSleepSchedule = async (sourceTimestamps: string[] = scheduleTimestamps, silent = false, active = controlMode === 'schedule') => {
    const resolvedSource = Array.isArray(sourceTimestamps) ? sourceTimestamps : scheduleTimestamps
    const timestamps = resolvedSource.map(v => v.trim())
    if (timestamps.some(v => !v)) {
      if (!silent) {
        setFeedback({ type: 'error', msg: '❌ Bitte alle Timestamp-Felder ausfüllen.' })
      }
      return
    }

    setSending(true)
    if (!silent) {
      setFeedback(null)
    }
    try {
      const r = await fetch(`${API}/sleep-schedule`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          count: timestamps.length,
          timestamps,
          actions: timestamps.map((_, index) => scheduleActions[index] ?? 'none'),
          awakeSeconds,
          active,
        }),
      })
      const data: SetCommandResponse = await r.json()
      if (data.ok) {
        setScheduleActive(active)
        if (!silent && data.stored) {
          setFeedback({ type: 'success', msg: `✅ ${timestamps.length} Timestamps gespeichert (wach: ${awakeSeconds}s)` })
        } else if (!silent) {
          setFeedback({ type: 'success', msg: `✅ ${timestamps.length} Timestamps gesendet (wach: ${awakeSeconds}s)` })
        }
        setTimeout(loadStatus, 500)
      } else {
        if (!silent) {
          setFeedback({ type: 'error', msg: `❌ ${data.error}` })
        }
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      if (!silent) {
        setFeedback({ type: 'error', msg: `❌ Fehler: ${message}` })
      }
    } finally {
      setSending(false)
    }
  }

  const activateManualMode = async () => {
    window.scrollTo({ top: 0, behavior: 'instant' })
    setControlMode('manual')
    setScheduleActive(false)
    await sendSleepSchedule(scheduleTimestamps, false, false)
  }

  const activateScheduleMode = () => {
    window.scrollTo({ top: 0, behavior: 'instant' })
    setControlMode('schedule')
  }

  const setScheduleEnabled = async (nextActive: boolean) => {
    setScheduleActive(nextActive)
    await sendSleepSchedule(scheduleTimestamps, false, nextActive)
  }

  const cardStyle: CSSProperties = {
    background: '#fff',
    borderRadius: 10,
    padding: '20px 24px',
    boxShadow: '0 1px 4px rgba(0,0,0,0.08)',
  }

  const btnStyle = (color = '#3b82f6'): CSSProperties => ({
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

  const ghostBtnStyle: CSSProperties = {
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

  const removeBtnStyle: CSSProperties = {
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

  const modeSwitchButton = (mode: ControlMode) => ({
    padding: '8px 14px',
    borderRadius: 999,
    border: controlMode === mode ? '1px solid #047857' : '1px solid #d1d5db',
    background: controlMode === mode ? '#ecfdf5' : '#ffffff',
    color: controlMode === mode ? '#047857' : '#374151',
    fontWeight: 700,
    fontSize: 13,
    cursor: sending ? 'wait' : 'pointer',
    opacity: sending ? 0.6 : 1,
  })

  const scheduleToggleTrackStyle: CSSProperties = {
    position: 'relative',
    width: 58,
    height: 32,
    borderRadius: 999,
    background: scheduleActive ? '#047857' : '#d1d5db',
    transition: 'background 0.2s ease',
    flexShrink: 0,
    boxShadow: scheduleActive ? 'inset 0 0 0 1px rgba(4, 120, 87, 0.35)' : 'inset 0 0 0 1px rgba(148, 163, 184, 0.35)',
  }

  const scheduleToggleThumbStyle: CSSProperties = {
    position: 'absolute',
    top: 3,
    left: scheduleActive ? 29 : 3,
    width: 26,
    height: 26,
    borderRadius: '50%',
    background: '#ffffff',
    boxShadow: '0 2px 6px rgba(15, 23, 42, 0.22)',
    transition: 'left 0.2s ease',
  }

  const formatStatusTimestamp = (unixMs: number | undefined) => {
    if (typeof unixMs === 'number' && Number.isFinite(unixMs) && unixMs > 0) {
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

  const scheduleHistoryRows = (() => {
    const entries = Array.isArray(status?.scheduleHistory)
      ? [...status.scheduleHistory].slice(-20).reverse()
      : []
    return Array.from({ length: 20 }, (_, idx) => entries[idx] ?? null)
  })()

  const scheduleEntryState = (entry: ScheduleHistoryEntry | null) => {
    if (!entry) {
      return 'leer'
    }
    if (typeof entry.wokeUpAtMs === 'number' && Number.isFinite(entry.wokeUpAtMs) && entry.wokeUpAtMs > 0) {
      return 'abgeschlossen'
    }
    if (typeof entry.sleepingAtMs === 'number' && Number.isFinite(entry.sleepingAtMs) && entry.sleepingAtMs > 0) {
      return 'schlaeft'
    }
    if (typeof entry.sleepCommandAtMs === 'number' && Number.isFinite(entry.sleepCommandAtMs) && entry.sleepCommandAtMs > 0) {
      return 'gesendet'
    }
    return 'offen'
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
            <StatusItem label="Akku" value={battery !== null && battery !== '' ? `${battery}%` : '—'} />
            <StatusItem label="Charging" value={charging ?? '—'} />
            <StatusItem label="IP" value={status.ip ?? '—'} />
            <StatusItem label="Position" value={status.position ?? '—'} />
            <StatusItem label="Letzte Aktion" value={status.lastAction ?? '—'} />
            <StatusItem label="Controller" value={status.controllerState ?? '—'} />
            <StatusItem label="Sleep-ACK" value={status.sleepState ?? '—'} />
            <StatusItem label="Weckgrund" value={wakeReason ?? '—'} />
            <StatusItem label="Schedule aktiv" value={status.scheduleActive ? 'ja' : 'nein'} />
            <StatusItem label="Schedule-Zeitzone" value={status.scheduleTimezone ?? '—'} />
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
        <div style={{ display: 'flex', gap: 8, marginBottom: 14, flexWrap: 'wrap' }}>
          <button type="button" style={modeSwitchButton('manual')} onClick={activateManualMode} disabled={sending}>
            Manuelle Steuerung aktiv
          </button>
          <button type="button" style={modeSwitchButton('schedule')} onClick={activateScheduleMode} disabled={sending}>
            Sleep-Schedule aktiv
          </button>
        </div>
          <p style={{ marginTop: -6, marginBottom: 12, color: '#6b7280', fontSize: 12 }}>
            Wechsel auf "Manuell" deaktiviert sofort. Aktivierung erfolgt erst mit "Timestamp-Schedule senden".
          </p>

        <div style={{ marginTop: 14, display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
          <label style={{ fontSize: 14, color: '#6b7280' }}>
            Motor Auto-Stop (Sekunden, 1-60):
            <input
              type="number"
              min={1}
              max={60}
              value={motorAutoStopSeconds}
              onChange={e => setMotorAutoStopSeconds(Math.max(1, Math.min(60, Number(e.target.value) || 1)))}
              style={{ marginLeft: 8, padding: '6px 8px', borderRadius: 6, border: '1px solid #e5e7eb', width: 80 }}
            />
          </label>
          <span style={{ fontSize: 12, color: '#6b7280' }}>
            Wenn der Motor laeuft, sendet das Backend nach Ablauf automatisch "stop".
          </span>
        </div>

        {controlMode === 'manual' && (
        <div>
          <h4 style={{ marginTop: 16, marginBottom: 10, color: '#374151' }}>Manuelle Steuerung</h4>
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
        )}

        {controlMode === 'schedule' && (
        <div>
          <h4 style={{ marginTop: 16, marginBottom: 8, color: selectedTheme.titleColor, fontSize: 18 }}>Sleep-Schedule per Timestamps</h4>
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
                    <select
                      value={scheduleActions[index] ?? 'none'}
                      onChange={(e) => updateScheduleAction(index, e.target.value as ScheduleAction)}
                      aria-label={`Aktion für Timestamp ${index + 1}`}
                      style={{ padding: '8px 10px', borderRadius: 10, border: `1px solid ${selectedTheme.inputBorder}`, background: selectedTheme.inputBg, color: selectedTheme.labelColor, minWidth: 145 }}
                    >
                      <option value="open">Öffnen</option>
                      <option value="close">Schließen</option>
                      <option value="stop">Stop</option>
                      <option value="none">Keine Aktion</option>
                    </select>
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
              onClick={() => setScheduleEnabled(true)}
            >
              Timestamp-Schedule senden
            </button>
          </div>

          <div style={{ marginTop: 14, display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
            <span style={{ fontSize: 13, color: selectedTheme.subtitleColor, fontWeight: 600 }}>Modus</span>
            <span style={{ fontSize: 13, color: selectedTheme.labelColor }}>Normal</span>
            <label
              style={{ display: 'inline-flex', alignItems: 'center', cursor: sending ? 'wait' : 'pointer', opacity: sending ? 0.6 : 1 }}
              title="Zwischen Normal und Schedule umschalten"
            >
              <input
                type="checkbox"
                checked={scheduleActive}
                onChange={(e) => {
                  if (e.target.checked) {
                    setScheduleEnabled(true)
                  } else {
                    setScheduleEnabled(false)
                  }
                }}
                disabled={sending}
                aria-label="Schedule-Modus aktivieren"
                style={{ position: 'absolute', opacity: 0, width: 1, height: 1, pointerEvents: 'none' }}
              />
              <span style={scheduleToggleTrackStyle}>
                <span style={scheduleToggleThumbStyle} />
              </span>
            </label>
            <span style={{ fontSize: 13, color: selectedTheme.labelColor }}>Schedule</span>
          </div>

          <div style={{ marginTop: 14 }}>
            <button
              type="button"
              onClick={() => setHistoryExpanded(prev => !prev)}
              style={{
                ...ghostBtnStyle,
                background: selectedTheme.rowBg,
                border: `1px solid ${selectedTheme.rowBorder}`,
                color: selectedTheme.labelColor,
              }}
            >
              {historyExpanded ? 'Verlauf ausblenden' : 'Verlauf anzeigen'} (letzte 20)
            </button>
          </div>

          {historyExpanded && (
            <div style={{ marginTop: 12, overflowX: 'auto', border: `1px solid ${selectedTheme.rowBorder}`, borderRadius: 10 }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13, color: selectedTheme.labelColor }}>
                <thead>
                  <tr style={{ background: selectedTheme.rowBg }}>
                    <th style={{ textAlign: 'left', padding: '10px 12px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>#</th>
                    <th style={{ textAlign: 'left', padding: '10px 12px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>Status</th>
                    <th style={{ textAlign: 'left', padding: '10px 12px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>Akku (%)</th>
                    <th style={{ textAlign: 'left', padding: '10px 12px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>Sleep (Sek.)</th>
                    <th style={{ textAlign: 'left', padding: '10px 12px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>Sleep gesendet um</th>
                    <th style={{ textAlign: 'left', padding: '10px 12px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>Geschlafen um</th>
                    <th style={{ textAlign: 'left', padding: '10px 12px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>Wach geworden um</th>
                  </tr>
                </thead>
                <tbody>
                  {scheduleHistoryRows.filter(entry => entry !== null).map((entry, idx) => (
                    <tr key={idx}>
                      <td style={{ padding: '8px 12px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{idx + 1}</td>
                      <td style={{ padding: '8px 12px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{scheduleEntryState(entry)}</td>
                      <td style={{ padding: '8px 12px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{entry?.batteryPercent ?? '—'}</td>
                      <td style={{ padding: '8px 12px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{entry?.sleepSeconds ?? '—'}</td>
                      <td style={{ padding: '8px 12px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{formatStatusTimestamp(entry?.sleepCommandAtMs)}</td>
                      <td style={{ padding: '8px 12px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{formatStatusTimestamp(entry?.sleepingAtMs)}</td>
                      <td style={{ padding: '8px 12px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{formatStatusTimestamp(entry?.wokeUpAtMs)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
        )}
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
