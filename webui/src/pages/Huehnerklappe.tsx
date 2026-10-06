import type { CSSProperties } from 'react'
import { useEffect, useRef, useState } from 'react'
import PageHeader, { pageHeaderButtonStyle } from '../components/PageHeader'

const API = '/api/huehnerklappe'

type ControlMode = 'manual' | 'schedule'
type ControlTab = ControlMode | 'test'
type ScheduleAction = 'open' | 'close' | 'stop' | 'none'
type MotorAction = 'open' | 'close'

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
  endPosition?: string
  motorDurationSec?: number
}

/** @brief Test-mode movement and sleep lifecycle fields returned by the backend. */
type TestModeHistoryEntry = {
  /** @brief Issued alternating motor direction, either open or close. */
  action?: string
  /** @brief Unix timestamp in milliseconds when the movement started. */
  actionAtMs?: number
  /** @brief Raw battery payload captured when the movement started. */
  batteryPercent?: string
  /** @brief Final door position after motor movement completed. */
  endPosition?: string
  /** @brief Measured motor runtime in seconds. */
  motorDurationSec?: number
  /** @brief Configured maximum-awake duration, including motor runtime floor. */
  maxAwakeSeconds?: number
  /** @brief Sleep duration sent to the controller in seconds. */
  sleepSeconds?: number
  /** @brief Unix timestamp in milliseconds when sleep was commanded. */
  sleepCommandAtMs?: number
  /** @brief Unix timestamp in milliseconds when sleeping was acknowledged. */
  sleepingAtMs?: number
  /** @brief Unix timestamp in milliseconds when the controller woke online. */
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
  limitClose?: string
  limitOpen?: string
  scheduleActive?: boolean
  scheduleTimezone?: string
  serverNowMs?: number
  sleepCommandAtMs?: number
  sleepingAtMs?: number
  onlineAtMs?: number
  wakeDeltaMs?: number
  scheduleHistory?: ScheduleHistoryEntry[]
  testModeHistory?: TestModeHistoryEntry[]
  testModeEnabled?: boolean
  testModeInWindow?: boolean
  testModeNextAction?: string
  testModeNextAt?: string
  testModeSleepPending?: boolean
  testModeState?: string
}

type UiStateResponse = {
  sleepTime?: number
  motorAutoStopSeconds?: number
  motorAutoStopOpenSeconds?: number
  motorAutoStopCloseSeconds?: number
  engineMaxRuntimeSeconds?: number
  sleepUntil?: string
  controlMode?: ControlMode
  scheduleActive?: boolean
  scheduleTimestamps?: string[]
  scheduleEntries?: Array<{ timestamp?: string; action?: ScheduleAction }>
  awakeSeconds?: number
  historyExpanded?: boolean
  testModeEnabled?: boolean
  testModeIntervalMinutes?: number
  testModeStartTime?: string
  testModeEndTime?: string
  testModeMaxAwakeSeconds?: number
}

type SetCommandResponse = {
  ok?: boolean
  error?: string
  stored?: boolean
  topic?: string
}

type PickerDraft = {
  hour: string
  minute: string
  second: string
}

/**
 * @brief Converts the backend test-mode lifecycle state into a concise German label.
 * @param state Raw status enum returned by the ChickenDoor status endpoint.
 * @return Human-readable status for the device status panel.
 */
function testModeStatusLabel(state?: string): string {
  switch (state) {
    case 'active': return 'aktiv'
    case 'paused_schedule': return 'pausiert (Schedule aktiv)'
    case 'outside_window': return 'außerhalb Zeitfenster'
    case 'waiting_controller': return 'warte auf Controller'
    case 'disabled': return 'inaktiv'
    default: return '—'
  }
}

/**
 * @brief Converts the next test-mode direction into its German UI label.
 * @param action Next normalized motor action returned by the backend.
 * @return German action text, defaulting to opening before the first action.
 */
function testModeActionLabel(action?: string): string {
  if (action === 'close') return 'Schließen'
  return 'Öffnen'
}

export default function Huehnerklappe() {
  const [sleepTime, setSleepTime] = useState(60) // default 60 Sekunden
  const [motorAutoStopOpenSeconds, setMotorAutoStopOpenSeconds] = useState(15)
  const [motorAutoStopCloseSeconds, setMotorAutoStopCloseSeconds] = useState(15)
  const [engineMaxRuntimeSeconds, setEngineMaxRuntimeSeconds] = useState(60)
  const [engineMaxRuntimeDraft, setEngineMaxRuntimeDraft] = useState(60)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [savingEngineMaxRuntime, setSavingEngineMaxRuntime] = useState(false)
  const [sleepUntil, setSleepUntil] = useState('')
  const [controlMode, setControlMode] = useState<ControlMode>('manual')
  const [controlTab, setControlTab] = useState<ControlTab>('manual')
  const [scheduleActive, setScheduleActive] = useState(false)
  const [scheduleTimestamps, setScheduleTimestamps] = useState(['06:30:00', '12:00:00', '18:30:00'])
  const [scheduleActions, setScheduleActions] = useState<ScheduleAction[]>(['none', 'none', 'none'])
  const [awakeSeconds, setAwakeSeconds] = useState(30)
  const [pickerIndex, setPickerIndex] = useState<number | null>(null)
  const [pickerDraft, setPickerDraft] = useState<PickerDraft>({ hour: '00', minute: '00', second: '00' })
  const [historyExpanded, setHistoryExpanded] = useState(false)
  const [testModeEnabled, setTestModeEnabled] = useState(false)
  const [testModeIntervalMinutes, setTestModeIntervalMinutes] = useState(30)
  const [testModeStartTime, setTestModeStartTime] = useState('08:00')
  const [testModeEndTime, setTestModeEndTime] = useState('20:00')
  const [testModeMaxAwakeSeconds, setTestModeMaxAwakeSeconds] = useState(30)
  const [status, setStatus] = useState<HuehnerklappeStatus | null>(null)
  const [optimisticMotorAction, setOptimisticMotorAction] = useState<MotorAction | null>(null)
  const [sending, setSending] = useState(false)
  const [feedback, setFeedback] = useState<Feedback | null>(null)
  const [battery, setBattery] = useState<string | null>(null)
  const [wakeReason, setWakeReason] = useState<string | null>(null)
  const [charging, setCharging] = useState<string | null>(null)
  const [clockOffsetMs, setClockOffsetMs] = useState(0)
  const [uiLoaded, setUiLoaded] = useState(false)
  const [settingsSaveError, setSettingsSaveError] = useState('')
  const uiStateSaveQueueRef = useRef<Promise<void>>(Promise.resolve())
  const uiStateSaveRevisionRef = useRef(0)

  // Status laden
  const loadStatus = async () => {
    try {
      const r = await fetch(`${API}/status`)
      if (r.ok) {
        const data: HuehnerklappeStatus = await r.json()
        setStatus(data)
        setOptimisticMotorAction(current => {
          if (!current) return null
          const action = String(data.lastAction ?? '').toLowerCase().trim()
          const actionMatches = current === 'open'
            ? action.includes('open') || action.includes('offen') || action.includes('auf')
            : action.includes('close') || action.includes('geschlossen') || action.includes('zu')
          return actionMatches ? null : current
        })
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
      const legacyMotorRuntime = Number.isFinite(data.motorAutoStopSeconds)
        ? Math.max(1, Math.min(60, Number(data.motorAutoStopSeconds)))
        : null
      if (Number.isFinite(data.motorAutoStopOpenSeconds)) {
        setMotorAutoStopOpenSeconds(Math.max(1, Math.min(60, Number(data.motorAutoStopOpenSeconds))))
      } else if (legacyMotorRuntime !== null) {
        setMotorAutoStopOpenSeconds(legacyMotorRuntime)
      }
      if (Number.isFinite(data.motorAutoStopCloseSeconds)) {
        setMotorAutoStopCloseSeconds(Math.max(1, Math.min(60, Number(data.motorAutoStopCloseSeconds))))
      } else if (legacyMotorRuntime !== null) {
        setMotorAutoStopCloseSeconds(legacyMotorRuntime)
      }
      if (Number.isFinite(data.engineMaxRuntimeSeconds) && Number(data.engineMaxRuntimeSeconds) >= 1) {
        setEngineMaxRuntimeSeconds(Math.min(60, Number(data.engineMaxRuntimeSeconds)))
      }
      if (typeof data.sleepUntil === 'string') {
        setSleepUntil(data.sleepUntil)
      }
      if (data.controlMode === 'manual' || data.controlMode === 'schedule') {
        setControlMode(data.controlMode)
        setControlTab(data.controlMode)
      } else if (data.scheduleActive) {
        setControlMode('schedule')
        setControlTab('schedule')
      } else {
        setControlMode('manual')
        setControlTab('manual')
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
      if (typeof data.testModeEnabled === 'boolean') {
        setTestModeEnabled(data.testModeEnabled)
      }
      if (Number.isFinite(data.testModeIntervalMinutes) && Number(data.testModeIntervalMinutes) >= 1) {
        setTestModeIntervalMinutes(Math.min(1440, Number(data.testModeIntervalMinutes)))
      }
      if (typeof data.testModeStartTime === 'string' && data.testModeStartTime) {
        setTestModeStartTime(data.testModeStartTime)
      }
      if (typeof data.testModeEndTime === 'string' && data.testModeEndTime) {
        setTestModeEndTime(data.testModeEndTime)
      }
      if (Number.isFinite(data.testModeMaxAwakeSeconds) && Number(data.testModeMaxAwakeSeconds) >= 1) {
        setTestModeMaxAwakeSeconds(Math.min(86400, Number(data.testModeMaxAwakeSeconds)))
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
      motorAutoStopOpenSeconds,
      motorAutoStopCloseSeconds,
      engineMaxRuntimeSeconds,
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
      testModeEnabled,
      testModeIntervalMinutes,
      testModeStartTime,
      testModeEndTime,
      testModeMaxAwakeSeconds,
    }

    const revision = ++uiStateSaveRevisionRef.current
    const timer = window.setTimeout(() => {
      // Serialize autosaves so a slower old request cannot overwrite a newer
      // setting snapshot in the durable backend state.
      uiStateSaveQueueRef.current = uiStateSaveQueueRef.current.then(async () => {
        try {
          const response = await fetch(`${API}/ui-state`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          })
          const result: { ok?: boolean; error?: string } = await response.json()
          if (!response.ok || result.ok !== true) {
            throw new Error(result.error ?? `HTTP ${response.status}`)
          }
          if (revision === uiStateSaveRevisionRef.current) {
            setSettingsSaveError('')
          }
        } catch (error) {
          if (revision === uiStateSaveRevisionRef.current) {
            const message = error instanceof Error ? error.message : String(error)
            setSettingsSaveError(`Einstellungen konnten nicht dauerhaft gespeichert werden: ${message}`)
          }
        }
      })
    }, 250)

    return () => window.clearTimeout(timer)
  }, [uiLoaded, sleepTime, motorAutoStopOpenSeconds, motorAutoStopCloseSeconds, engineMaxRuntimeSeconds, sleepUntil, controlMode, scheduleActive, scheduleTimestamps, scheduleActions, awakeSeconds, historyExpanded, testModeEnabled, testModeIntervalMinutes, testModeStartTime, testModeEndTime, testModeMaxAwakeSeconds])

  const sendCommand = async (key: string, value: string | number | null = null, successMessage: string | null = null) => {
    const motorAction = key === 'engine' && (value === 'open' || value === 'close') ? value : null
    if (key === 'engine') {
      setOptimisticMotorAction(motorAction)
    }
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
        if (motorAction) setOptimisticMotorAction(null)
        setFeedback({ type: 'error', msg: `❌ ${data.error}` })
      }
      setTimeout(loadStatus, 1000)
    } catch (err) {
      if (motorAction) setOptimisticMotorAction(null)
      const message = err instanceof Error ? err.message : String(err)
      setFeedback({ type: 'error', msg: `❌ Fehler: ${message}` })
    } finally {
      setSending(false)
    }
  }

  /**
   * @brief Publishes and stores the shared maximum motor runtime.
   * @details The API validates the 1–60 second value, publishes it to the
   * dedicated controller configuration topic, and persists it in bbolt only
   * after the MQTT broker accepts the publication.
   * @param seconds Draft maximum runtime in whole seconds.
   * @return Resolves after showing success or failure feedback to the user.
   */
  const saveEngineMaxRuntime = async (seconds: number) => {
    setSavingEngineMaxRuntime(true)
    setFeedback(null)
    try {
      // Run the explicit MQTT-backed save in the same queue as complete UI
      // snapshots. This prevents an older autosave from persisting a stale
      // engineMaxRuntime value after the dedicated endpoint has accepted the
      // user's new maximum.
      const saveRequest = uiStateSaveQueueRef.current.then(async () => {
        const response = await fetch(`${API}/set`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ key: 'engineMaxRuntime', value: seconds }),
        })
        const result: SetCommandResponse = await response.json()
        if (!response.ok || !result.ok) {
          throw new Error(result.error ?? 'Maximale Motorlaufzeit konnte nicht gespeichert werden.')
        }
        return result
      })
      uiStateSaveQueueRef.current = saveRequest.then(() => undefined, () => undefined)
      const result = await saveRequest
      setEngineMaxRuntimeSeconds(seconds)
      setSettingsOpen(false)
      setFeedback({ type: 'success', msg: `✅ Maximale Motorlaufzeit ${seconds} s gespeichert und an ${result.topic ?? 'MQTT'} gesendet.` })
    } catch (error) {
      setFeedback({ type: 'error', msg: `❌ ${error instanceof Error ? error.message : String(error)}` })
    } finally {
      setSavingEngineMaxRuntime(false)
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
    setControlMode('manual')
    setScheduleActive(false)
    await sendSleepSchedule(scheduleTimestamps, false, false)
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

  const formatPositionLabel = (pos?: string) => {
    if (!pos || pos === '—' || pos === '-') return '—'
    const p = pos.toLowerCase().trim()
    if (p === 'open' || p === 'offen' || p === 'auf') return 'offen'
    if (p === 'closed' || p === 'close' || p === 'geschlossen' || p === 'schließen' || p === 'schliessen' || p === 'zu') return 'geschlossen'
    if (p === 'stop') return 'gestoppt'
    if (p === 'in bewegung' || p === 'moving' || p === 'fahrt' || p === 'laeuft') return 'in Bewegung'
    if (p === 'zwischenposition') return 'Zwischenposition'
    if (p === 'ok') return 'bereit'
    return pos
  }

  const scheduleHistoryRows = (() => {
    const entries = Array.isArray(status?.scheduleHistory)
      ? [...status.scheduleHistory].slice(-20).reverse()
      : []
    return Array.from({ length: 20 }, (_, idx) => entries[idx] ?? null)
  })()

  const testModeHistoryRows = (() => {
    const entries = Array.isArray(status?.testModeHistory)
      ? [...status.testModeHistory].slice(-20).reverse()
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
    <div style={{ width: '100%', maxWidth: 1200, minWidth: 0, margin: '0 auto', boxSizing: 'border-box' }}>
      <PageHeader
        eyebrow="MQTT / HÜHNERKLAPPE"
        title="Motor"
        subtitle="Hühnerklappe und Steuerung"
        actions={<button
          type="button"
          onClick={() => {
            setEngineMaxRuntimeDraft(engineMaxRuntimeSeconds)
            setFeedback(null)
            setSettingsOpen(true)
          }}
          style={pageHeaderButtonStyle('transparent', '#fff')}
        >
          ⚙ Einstellungen
        </button>}
      />

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
      {settingsSaveError && (
        <div role="alert" style={{ padding: '10px 16px', borderRadius: 8, marginBottom: 16, background: '#fee2e2', color: '#991b1b', fontWeight: 500 }}>
          ❌ {settingsSaveError}
        </div>
      )}

      {/* Status */}
      <div style={{ ...cardStyle, marginBottom: 16 }}>
        <h3 style={{ marginTop: 0, color: '#374151' }}>📊 Aktueller Status</h3>
        {status ? (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(min(100%, 210px), 1fr))', gap: 12 }}>
            <StatusItem label="Akku" value={battery !== null && battery !== '' ? `${battery}%` : '—'} />
            <StatusItem label="Charging" value={charging ?? '—'} />
            <StatusItem label="IP" value={status.ip ?? '—'} />
            <StatusItem label="Position" value={formatPositionLabel(status.position)} />
            <StatusItem label="Endschalter ZU" value={status.limitClose ?? '—'} />
            <StatusItem label="Endschalter AUF" value={status.limitOpen ?? '—'} />
            <StatusItem label="Letzte Aktion" value={status.lastAction ?? '—'} />
            <StatusItem label="Controller" value={status.controllerState ?? '—'} />
            <StatusItem label="Sleep-ACK" value={status.sleepState ?? '—'} />
            <StatusItem label="Weckgrund" value={wakeReason ?? '—'} />
            <StatusItem label="Schedule aktiv" value={status.scheduleActive ? 'ja' : 'nein'} />
            <StatusItem label="Testmodus" value={testModeStatusLabel(status.testModeState)} />
            <StatusItem label="Nächste Testaktion" value={status.testModeEnabled ? `${testModeActionLabel(status.testModeNextAction)} · ${status.testModeNextAt || 'Intervall läuft'}` : '—'} />
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

      <ChickenDoorGraphic status={status} optimisticAction={optimisticMotorAction} />

      {/* Steuerung */}
      <div style={{ ...cardStyle }}>
        <h3 style={{ marginTop: 0, color: '#374151' }}>🔧 Klappe steuern</h3>
        <div role="tablist" aria-label="Klappensteuerung" style={{ display: 'flex', gap: 0, borderBottom: '1px solid #d1d5db', marginBottom: 18 }}>
          {([
            { id: 'manual', label: 'Manuelle Steuerung' },
            { id: 'schedule', label: 'Sleep-Schedule' },
            { id: 'test', label: 'Testmodus' },
          ] as const).map((tab) => (
            <button
              key={tab.id}
              type="button"
              role="tab"
              id={`chicken-door-tab-${tab.id}`}
              aria-selected={controlTab === tab.id}
              aria-controls={`chicken-door-panel-${tab.id}`}
              onClick={() => {
                setControlTab(tab.id)
                if (tab.id === 'manual' || tab.id === 'schedule') {
                  // Selecting a tab only changes the visible panel and its
                  // persisted UI preference; schedule activation stays explicit.
                  setControlMode(tab.id)
                }
              }}
              style={{
                padding: '10px 16px',
                border: 'none',
                borderBottom: controlTab === tab.id ? '3px solid #b91c1c' : '3px solid transparent',
                background: 'transparent',
                color: controlTab === tab.id ? '#263d52' : '#6b7280',
                fontWeight: controlTab === tab.id ? 700 : 600,
                cursor: 'pointer',
              }}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {controlTab === 'test' && (
          <div id="chicken-door-panel-test" role="tabpanel" aria-labelledby="chicken-door-tab-test">
            <div style={{ padding: 14, border: '1px solid #d1d5db', borderRadius: 8, background: '#f9fafb' }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' }}>
                <div>
                  <strong style={{ color: '#374151' }}>Zyklischer Klappentest</strong>
                  <p style={{ margin: '4px 0 0', color: '#6b7280', fontSize: 12 }}>
                    Nach jeder Bewegung schläft der Controller für das eingestellte Intervall und wacht zur nächsten wechselnden Öffnen-/Schließen-Aktion auf. Ein aktiver Timestamp-Schedule pausiert den Testmodus.
                  </p>
                </div>
                <button
                  type="button"
                  aria-pressed={testModeEnabled}
                  onClick={() => setTestModeEnabled((enabled) => !enabled)}
                  style={modeSwitchButton(testModeEnabled ? 'schedule' : 'manual')}
                >
                  {testModeEnabled ? 'Testmodus aktiv' : 'Testmodus inaktiv'}
                </button>
              </div>
              <div style={{ marginTop: 12, display: 'flex', gap: 14, alignItems: 'center', flexWrap: 'wrap' }}>
                <label style={{ fontSize: 14, color: '#6b7280' }}>
                  Abstand (Minuten, 1–1440):
                  <input
                    type="number"
                    min={1}
                    max={1440}
                    value={testModeIntervalMinutes}
                    onChange={e => setTestModeIntervalMinutes(Math.max(1, Math.min(1440, Number(e.target.value) || 1)))}
                    style={{ marginLeft: 8, padding: '6px 8px', borderRadius: 6, border: '1px solid #e5e7eb', width: 88 }}
                  />
                </label>
                <label style={{ fontSize: 14, color: '#6b7280' }}>
                  Start:
                  <input
                    type="time"
                    value={testModeStartTime}
                    onChange={e => setTestModeStartTime(e.target.value)}
                    style={{ marginLeft: 8, padding: '6px 8px', borderRadius: 6, border: '1px solid #e5e7eb' }}
                  />
                </label>
                <label style={{ fontSize: 14, color: '#6b7280' }}>
                  Ende:
                  <input
                    type="time"
                    value={testModeEndTime}
                    onChange={e => setTestModeEndTime(e.target.value)}
                    style={{ marginLeft: 8, padding: '6px 8px', borderRadius: 6, border: '1px solid #e5e7eb' }}
                  />
                </label>
                <label style={{ fontSize: 14, color: '#6b7280' }}>
                  Max. Wachzeit (Sekunden):
                  <input
                    type="number"
                    min={1}
                    max={86400}
                    value={testModeMaxAwakeSeconds}
                    onChange={e => setTestModeMaxAwakeSeconds(Math.max(1, Math.min(86400, Number(e.target.value) || 1)))}
                    style={{ marginLeft: 8, padding: '6px 8px', borderRadius: 6, border: '1px solid #e5e7eb', width: 100 }}
                  />
                </label>
                <label style={{ fontSize: 14, color: '#6b7280' }}>
                  Auto-Stop Öffnen (Sekunden, 1–60):
                  <input
                    type="number"
                    min={1}
                    max={60}
                    value={motorAutoStopOpenSeconds}
                    onChange={e => setMotorAutoStopOpenSeconds(Math.max(1, Math.min(60, Number(e.target.value) || 1)))}
                    style={{ marginLeft: 8, padding: '6px 8px', borderRadius: 6, border: '1px solid #e5e7eb', width: 80 }}
                  />
                </label>
                <label style={{ fontSize: 14, color: '#6b7280' }}>
                  Auto-Stop Schließen (Sekunden, 1–60):
                  <input
                    type="number"
                    min={1}
                    max={60}
                    value={motorAutoStopCloseSeconds}
                    onChange={e => setMotorAutoStopCloseSeconds(Math.max(1, Math.min(60, Number(e.target.value) || 1)))}
                    style={{ marginLeft: 8, padding: '6px 8px', borderRadius: 6, border: '1px solid #e5e7eb', width: 80 }}
                  />
                </label>
              </div>
              <p style={{ margin: '10px 0 0', color: '#6b7280', fontSize: 12 }}>
                Zeitfenster verwendet die Schedule-Zeitzone; Start ist eingeschlossen, Ende ausgeschlossen. Über Mitternacht laufende Zeitfenster sind möglich. Die Wachzeit wird bei Bedarf auf die Motorlaufzeit angehoben. Die Auto-Stop-Werte werden mit manueller Steuerung und Schedule geteilt.
              </p>
              <div style={{ marginTop: 14, padding: '10px 12px', border: '1px solid #d1d5db', borderRadius: 6, background: '#fff' }}>
                <strong style={{ color: '#374151', fontSize: 13 }}>Testlauf-Status: </strong>
                <span style={{ color: '#374151', fontSize: 13 }}>
                  {testModeStatusLabel(status?.testModeState)}
                </span>
                <div style={{ marginTop: 4, color: '#6b7280', fontSize: 12 }}>
                  {testModeEnabled
                    ? status?.testModeSleepPending
                      ? `Nächste Aktion: ${testModeActionLabel(status.testModeNextAction)} – nach der Maximalwachzeit schläft der Controller bis zum nächsten Intervall.`
                      : `Nächste Aktion: ${testModeActionLabel(status?.testModeNextAction)}${status?.testModeNextAt ? ` um ${status.testModeNextAt}` : ' – Intervall wird gestartet'}. Der Controller schläft zwischen den Aktionen; der Timestamp-Schedule muss aus sein.`
                    : 'Zum Starten den Testmodus oben aktivieren.'}
                </div>
              </div>
            </div>
            <div style={{ marginTop: 16 }}>
              <h4 style={{ margin: '0 0 10px', color: '#374151' }}>Testmodus-Verlauf (letzte 20 Zyklen)</h4>
              <div style={{ overflowX: 'auto', border: `1px solid ${selectedTheme.rowBorder}`, borderRadius: 8 }}>
                <table style={{ width: '100%', minWidth: 940, borderCollapse: 'collapse', fontSize: 12, color: selectedTheme.labelColor }}>
                  <thead>
                    <tr style={{ background: selectedTheme.rowBg }}>
                      {['#', 'Aktion', 'Start', 'Endposition', 'Motorlaufzeit', 'Akku (%)', 'Max. wach (s)', 'Sleep (s)', 'Sleep gesendet', 'Geschlafen um', 'Aufgewacht um'].map((label) => (
                        <th key={label} style={{ textAlign: 'left', padding: '9px 10px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{label}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {testModeHistoryRows.map((entry, index) => (
                      <tr key={entry?.actionAtMs ?? `empty-${index}`}>
                        <td style={{ padding: '8px 10px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{entry ? index + 1 : '—'}</td>
                        <td style={{ padding: '8px 10px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{entry ? testModeActionLabel(entry.action) : '—'}</td>
                        <td style={{ padding: '8px 10px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{formatStatusTimestamp(entry?.actionAtMs)}</td>
                        <td style={{ padding: '8px 10px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{formatPositionLabel(entry?.endPosition)}</td>
                        <td style={{ padding: '8px 10px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{entry?.motorDurationSec ? `${entry.motorDurationSec.toFixed(1)} s` : '—'}</td>
                        <td style={{ padding: '8px 10px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{entry?.batteryPercent ?? '—'}</td>
                        <td style={{ padding: '8px 10px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{entry?.maxAwakeSeconds ?? '—'}</td>
                        <td style={{ padding: '8px 10px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{entry?.sleepSeconds ?? '—'}</td>
                        <td style={{ padding: '8px 10px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{formatStatusTimestamp(entry?.sleepCommandAtMs)}</td>
                        <td style={{ padding: '8px 10px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{formatStatusTimestamp(entry?.sleepingAtMs)}</td>
                        <td style={{ padding: '8px 10px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>{formatStatusTimestamp(entry?.wokeUpAtMs)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}

        {(controlTab === 'manual' || controlTab === 'schedule') && (
        <div>
        <div style={{ marginTop: 14, display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
          <label style={{ fontSize: 14, color: '#6b7280' }}>
            Auto-Stop Öffnen (Sekunden, 1-60):
            <input
              type="number"
              min={1}
              max={60}
              value={motorAutoStopOpenSeconds}
              onChange={e => setMotorAutoStopOpenSeconds(Math.max(1, Math.min(60, Number(e.target.value) || 1)))}
              style={{ marginLeft: 8, padding: '6px 8px', borderRadius: 6, border: '1px solid #e5e7eb', width: 80 }}
            />
          </label>
          <label style={{ fontSize: 14, color: '#6b7280' }}>
            Auto-Stop Schließen (Sekunden, 1-60):
            <input
              type="number"
              min={1}
              max={60}
              value={motorAutoStopCloseSeconds}
              onChange={e => setMotorAutoStopCloseSeconds(Math.max(1, Math.min(60, Number(e.target.value) || 1)))}
              style={{ marginLeft: 8, padding: '6px 8px', borderRadius: 6, border: '1px solid #e5e7eb', width: 80 }}
            />
          </label>
          <span style={{ fontSize: 12, color: '#6b7280' }}>
            Das Backend schreibt die Werte als runtime open bzw. runtime close und stoppt danach automatisch.
          </span>
        </div>

        {controlTab === 'manual' && (
        <div id="chicken-door-panel-manual" role="tabpanel" aria-labelledby="chicken-door-tab-manual">
          <p style={{ marginTop: 2, marginBottom: 12, color: '#6b7280', fontSize: 12 }}>
            Die Registerkarte ändert den aktiven Modus nicht. „Manuelle Steuerung aktiv“ deaktiviert den Timestamp-Schedule ausdrücklich.
          </p>
          <button type="button" style={modeSwitchButton('manual')} onClick={activateManualMode} disabled={sending}>
            Manuelle Steuerung aktivieren
          </button>
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
        </div>
        )}

        {controlTab === 'schedule' && (
        <div id="chicken-door-panel-schedule" role="tabpanel" aria-labelledby="chicken-door-tab-schedule">
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
            <button
              type="button"
              onClick={() => setScheduleEnabled(!scheduleActive)}
              disabled={sending}
              aria-label="Schedule-Modus aktivieren"
              aria-pressed={scheduleActive}
              title="Zwischen Normal und Schedule umschalten"
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                justifyContent: 'center',
                border: 'none',
                padding: 0,
                background: 'transparent',
                cursor: sending ? 'wait' : 'pointer',
                opacity: sending ? 0.6 : 1,
              }}
            >
              <span style={scheduleToggleTrackStyle}>
                <span style={scheduleToggleThumbStyle} />
              </span>
            </button>
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
                    <th style={{ textAlign: 'left', padding: '10px 12px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>Endposition</th>
                    <th style={{ textAlign: 'left', padding: '10px 12px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>Motorlaufzeit</th>
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
                      <td style={{ padding: '8px 12px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>
                        <span style={{
                          fontWeight: entry?.endPosition ? 700 : 'normal',
                          color: entry?.endPosition?.toLowerCase() === 'open' || entry?.endPosition?.toLowerCase() === 'offen' || entry?.endPosition?.toLowerCase() === 'auf'
                            ? '#059669'
                            : entry?.endPosition?.toLowerCase() === 'closed' || entry?.endPosition?.toLowerCase() === 'schließen' || entry?.endPosition?.toLowerCase() === 'geschlossen' || entry?.endPosition?.toLowerCase() === 'zu'
                            ? '#dc2626'
                            : selectedTheme.labelColor,
                        }}>
                          {formatPositionLabel(entry?.endPosition)}
                        </span>
                      </td>
                      <td style={{ padding: '8px 12px', borderBottom: `1px solid ${selectedTheme.rowBorder}` }}>
                        {typeof entry?.motorDurationSec === 'number' && entry.motorDurationSec > 0
                          ? `${entry.motorDurationSec.toFixed(1)} s`
                          : '—'}
                      </td>
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
        )}
      </div>
      {settingsOpen && (
        <EngineRuntimeSettingsModal
          value={engineMaxRuntimeDraft}
          saving={savingEngineMaxRuntime}
          errorMessage={feedback?.type === 'error' ? feedback.msg : ''}
          onChange={setEngineMaxRuntimeDraft}
          onClose={() => setSettingsOpen(false)}
          onSave={() => saveEngineMaxRuntime(engineMaxRuntimeDraft)}
        />
      )}
    </div>
  )
}

type EngineRuntimeSettingsModalProps = {
  /** @brief Current unsaved runtime draft, in seconds. */
  value: number
  /** @brief Indicates that the server is validating and publishing the setting. */
  saving: boolean
  /** @brief Error message from the most recent failed save, if any. */
  errorMessage: string
  /** @brief Updates the local, unsaved runtime draft. */
  onChange: (value: number) => void
  /** @brief Closes the dialog without changing the persisted setting. */
  onClose: () => void
  /** @brief Persists and publishes the currently selected runtime. */
  onSave: () => void
}

/**
 * @brief Renders the ChickenDoor controller maximum-runtime settings dialog.
 * @details The single value is a controller-wide safety ceiling, while the
 * existing opening/closing auto-stop values remain separate directional
 * limits. The backend applies the lower of both limits and publishes this
 * setting under nano/esp32/engineMaxRuntime after confirmation.
 * @param props Dialog value, save state, and interaction callbacks.
 * @return Accessible modal dialog containing the maximum runtime control.
 */
function EngineRuntimeSettingsModal({ value, saving, errorMessage, onChange, onClose, onSave }: EngineRuntimeSettingsModalProps) {
  return (
    <div role="dialog" aria-modal="true" aria-labelledby="chicken-door-settings-title" style={{ position: 'fixed', inset: 0, zIndex: 30, background: 'rgba(19, 39, 43, 0.48)', display: 'grid', placeItems: 'center', padding: 20 }}>
      <div style={{ width: 'min(100%, 480px)', background: '#fff', borderRadius: 12, padding: 24, boxShadow: '0 20px 60px rgba(0,0,0,0.22)', color: '#273746', fontFamily: 'Arial, sans-serif' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12 }}>
          <h2 id="chicken-door-settings-title" style={{ margin: 0, color: '#263d52', fontFamily: 'Georgia, "Times New Roman", serif' }}>Hühnerklappen-Einstellungen</h2>
          <button type="button" onClick={onClose} aria-label="Einstellungen schließen" disabled={saving} style={{ border: 0, background: 'transparent', fontSize: 24, cursor: 'pointer', color: '#64748b' }}>×</button>
        </div>
        <p style={{ color: '#64748b', fontSize: 13, lineHeight: 1.5 }}>Die Einstellung wird serverseitig gespeichert und auf dem MQTT-Topic <strong>nano/esp32/engineMaxRuntime</strong> in Sekunden an den Controller gesendet.</p>
        {errorMessage && <div role="alert" style={{ marginTop: 14, padding: '10px 12px', borderRadius: 6, background: '#fee2e2', color: '#991b1b', fontSize: 13 }}>{errorMessage}</div>}
        <label style={{ display: 'grid', gap: 7, marginTop: 18, color: '#365065', fontSize: 13, fontWeight: 700 }}>
          Maximale Motorlaufzeit (Sekunden, 1–60)
          <input type="number" min={1} max={60} step={1} value={value} disabled={saving} onChange={event => onChange(Math.max(1, Math.min(60, Number(event.target.value) || 1)))} style={{ width: '100%', boxSizing: 'border-box', border: '1px solid #cbd5df', borderRadius: 6, padding: '10px 11px', fontSize: 15, color: '#20343a', background: '#fbfdff' }} />
          <span style={{ color: '#718392', fontSize: 12, fontWeight: 400 }}>Wirkt als gemeinsame Obergrenze zusätzlich zu den getrennten Auto-Stop-Zeiten für Öffnen und Schließen.</span>
        </label>
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10, marginTop: 22 }}>
          <button type="button" onClick={onClose} disabled={saving} style={{ ...pageHeaderButtonStyle('#f3f7f7', '#31565c'), borderColor: '#cbd5df' }}>Abbrechen</button>
          <button type="button" onClick={onSave} disabled={saving} style={{ ...pageHeaderButtonStyle('#263d52', '#fff'), borderColor: '#263d52', opacity: saving ? 0.65 : 1 }}>{saving ? 'Speichert …' : 'Speichern & senden'}</button>
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

type ChickenDoorGraphicProps = {
  status: HuehnerklappeStatus | null
  optimisticAction: MotorAction | null
}

function ChickenDoorGraphic({ status, optimisticAction }: ChickenDoorGraphicProps) {
  const history = Array.isArray(status?.scheduleHistory) ? status.scheduleHistory : []
  const normalizedAction = String(optimisticAction ?? status?.lastAction ?? '').toLowerCase().trim()
  const normalizedLimitOpen = String(status?.limitOpen ?? '').toLowerCase().trim()
  const normalizedLimitClose = String(status?.limitClose ?? '').toLowerCase().trim()
  const activeLimitValues = ['active', 'on', '1', 'true', 'high', 'pressed', 'triggered', 'closed']
  const openLimitActive = activeLimitValues.includes(normalizedLimitOpen)
  const closeLimitActive = activeLimitValues.includes(normalizedLimitClose)
  const actionIsOpen = normalizedAction.includes('open') || normalizedAction.includes('offen') || normalizedAction.includes('auf')
  const actionIsClosed = normalizedAction.includes('close') || normalizedAction.includes('geschlossen') || normalizedAction.includes('zu')
  const targetOpen = openLimitActive && !closeLimitActive
  const motorIsMoving = normalizedAction.includes('open') || normalizedAction.includes('close') || normalizedAction.includes('oeffnen') || normalizedAction.includes('schliessen')
  const movementTargetOpen = actionIsOpen && !actionIsClosed
  const [visualOpen, setVisualOpen] = useState(targetOpen)
  const hasRendered = useRef(false)
  const frameColor = closeLimitActive && !openLimitActive ? '#dc2626' : openLimitActive && !closeLimitActive ? '#16a34a' : '#111827'

  const targetPosition = targetOpen ? 'open' : 'closed'
  const matchingEntry = [...history].reverse().find((entry) => {
    const position = String(entry.endPosition ?? '').toLowerCase()
    const matchesTarget = targetOpen
      ? position.includes('open') || position.includes('offen') || position.includes('auf')
      : position.includes('close') || position.includes('geschlossen') || position.includes('zu')
    return matchesTarget && typeof entry.motorDurationSec === 'number' && entry.motorDurationSec > 0
  })
  const animationDuration = Math.max(0.2, matchingEntry?.motorDurationSec ?? 1)

  useEffect(() => {
    if (!hasRendered.current) {
      hasRendered.current = true
      return
    }
    if (motorIsMoving) {
      setVisualOpen(movementTargetOpen)
    } else {
      setVisualOpen(targetOpen)
    }
  }, [motorIsMoving, movementTargetOpen, targetOpen, targetPosition])

  return (
    <div style={{
      background: '#fff',
      borderRadius: 10,
      padding: '20px 24px',
      marginBottom: 16,
      boxShadow: '0 1px 4px rgba(0,0,0,0.08)',
      overflow: 'hidden',
      position: 'relative',
      isolation: 'isolate',
    }}>
      <h3 style={{ marginTop: 0, color: '#374151' }}>🐔 Klappenansicht</h3>
      <div style={{ display: 'flex', justifyContent: 'center', padding: '8px 0 14px' }}>
        <div style={{
          position: 'relative',
          width: 'min(100%, 280px)',
          height: 'auto',
          aspectRatio: '1 / 1',
          background: '#f3f4f6',
          border: `3px solid ${frameColor}`,
          overflow: 'hidden',
          clipPath: 'inset(0)',
          boxSizing: 'border-box',
        }}>
          <div style={{
            position: 'absolute',
            top: 0,
            bottom: 0,
            left: 18,
            width: 6,
            background: '#fff',
            border: '2px solid #111827',
            boxSizing: 'border-box',
          }} />
          <div style={{
            position: 'absolute',
            top: 0,
            bottom: 0,
            right: 18,
            width: 6,
            background: '#fff',
            border: '2px solid #111827',
            boxSizing: 'border-box',
          }} />
          <div style={{
            position: 'absolute',
            bottom: 0,
            left: 24,
            width: 'calc(100% - 48px)',
            height: '100%',
            backgroundColor: '#b9783e',
            backgroundImage: [
              'linear-gradient(90deg, rgba(72, 35, 14, 0.20), transparent 12%, rgba(255, 222, 165, 0.18) 48%, rgba(72, 35, 14, 0.16))',
              'repeating-linear-gradient(0deg, transparent 0 34px, rgba(73, 39, 18, 0.48) 34px 37px, rgba(255, 224, 170, 0.22) 38px 40px)',
              'repeating-linear-gradient(90deg, rgba(255, 226, 175, 0.14) 0 2px, transparent 2px 38px, rgba(67, 35, 17, 0.20) 39px 41px, transparent 42px 76px)',
              'repeating-linear-gradient(0deg, #c8894d 0 68px, #ad6d38 68px 136px)',
            ].join(', '),
            border: '3px solid #70421f',
            boxShadow: 'inset 0 0 0 2px rgba(255, 220, 165, 0.35), inset 0 8px 14px rgba(67, 35, 17, 0.18)',
            boxSizing: 'border-box',
            transform: visualOpen ? 'translateY(-100%)' : 'translateY(0)',
            transition: motorIsMoving ? `transform ${animationDuration}s ease-in-out` : 'none',
          }} />
        </div>
      </div>
      <div
        role="status"
        aria-live="polite"
        style={{
          width: 'fit-content',
          margin: '0 auto',
          padding: '5px 11px',
          borderRadius: 4,
          background: visualOpen ? '#dcfce7' : '#fef3c7',
          color: visualOpen ? '#166534' : '#854d0e',
          fontSize: 12,
          fontWeight: 700,
          letterSpacing: '0.04em',
        }}
      >
        {visualOpen ? 'OFFEN' : 'GESCHLOSSEN'}
      </div>
    </div>
  )
}
