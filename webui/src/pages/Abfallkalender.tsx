import { useEffect, useState } from 'react'
import PageHeader, { pageHeaderButtonStyle } from '../components/PageHeader'

const API = '/api/waste-calendar'
const CALENDAR_URL = 'https://awido.cubefour.de/Customer/kelheim/v3/Calendar2.aspx'
const refreshOptions = [1, 2, 4, 8, 12, 26, 52]

type WasteCalendarStatus = {
  location: string
  calendarUrl: string
  instanceId: string
  reloadVersion: number
  refreshIntervalWeeks: number
  lastAttemptAt?: string
  lastSuccessAt?: string
  nextRefreshAt?: string
  refreshing: boolean
  error?: string
}

type ApiError = { error?: string }
type SettingsResponse = { ok?: boolean; settings?: { refreshIntervalWeeks: number }; error?: string }

/**
 * @brief Formats a calendar-service timestamp for the header status strip.
 * @param value ISO timestamp supplied by the backend.
 * @return Localized date and time or a neutral placeholder.
 */
function formatRefreshTime(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString('de-DE', { dateStyle: 'short', timeStyle: 'short' })
}

/**
 * @brief Displays the Landkreis Kelheim Awido waste calendar for Riedenburg-Jachenhausen.
 * @details The upstream widget owns its original calendar controls and visual layout. Refresh timing is persisted by the backend; its process identity and reload version force the iframe to reload on backend startup and scheduled refreshes.
 * @return Calendar page with refresh actions, settings, source status, and the original embedded calendar.
 */
export default function Abfallkalender() {
  const [status, setStatus] = useState<WasteCalendarStatus | null>(null)
  const [apiError, setApiError] = useState('')
  const [actionMessage, setActionMessage] = useState('')
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [draftIntervalWeeks, setDraftIntervalWeeks] = useState(1)
  const [savingSettings, setSavingSettings] = useState(false)
  const [requestingRefresh, setRequestingRefresh] = useState(false)

  const loadStatus = async () => {
    try {
      const response = await fetch(`${API}/status`)
      if (!response.ok) throw new Error(`HTTP ${response.status}`)
      const data: WasteCalendarStatus = await response.json()
      setStatus(data)
      setApiError('')
    } catch (error) {
      setApiError(error instanceof Error ? error.message : String(error))
    }
  }

  useEffect(() => {
    void loadStatus()
    const timer = window.setInterval(() => void loadStatus(), 15000)
    return () => window.clearInterval(timer)
  }, [])

  const refreshCalendar = async () => {
    setRequestingRefresh(true)
    setActionMessage('')
    try {
      const response = await fetch(`${API}/refresh`, { method: 'POST' })
      const result: ApiError & { ok?: boolean } = await response.json()
      if (!response.ok || result.ok !== true) throw new Error(result.error ?? `HTTP ${response.status}`)
      setActionMessage('Aktualisierung angefordert')
      window.setTimeout(() => void loadStatus(), 1000)
    } catch (error) {
      setActionMessage(`Aktualisierung fehlgeschlagen: ${error instanceof Error ? error.message : String(error)}`)
    } finally {
      setRequestingRefresh(false)
    }
  }

  const saveSettings = async () => {
    setSavingSettings(true)
    setActionMessage('')
    try {
      const response = await fetch(`${API}/settings`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refreshIntervalWeeks: draftIntervalWeeks }),
      })
      const result: SettingsResponse = await response.json()
      if (!response.ok || result.ok !== true || !result.settings) {
        throw new Error(result.error ?? `HTTP ${response.status}`)
      }
      setStatus(previous => previous
        ? { ...previous, refreshIntervalWeeks: result.settings!.refreshIntervalWeeks }
        : previous)
      setActionMessage('Aktualisierungsintervall gespeichert')
      setSettingsOpen(false)
    } catch (error) {
      setActionMessage(`Einstellung konnte nicht gespeichert werden: ${error instanceof Error ? error.message : String(error)}`)
    } finally {
      setSavingSettings(false)
    }
  }

  const statusLabel = !status
    ? 'BACKEND VERBINDEN'
    : status.error
      ? 'AWIDO-ABRUF FEHLGESCHLAGEN'
      : status.refreshing
        ? 'KALENDER WIRD AKTUALISIERT'
        : 'KALENDERDIENST BEREIT'
  const statusDetail = status
    ? `Riedenburg · Jachenhausen | zuletzt ${formatRefreshTime(status.lastSuccessAt)} | nächste Prüfung ${formatRefreshTime(status.nextRefreshAt)}`
    : apiError || 'Kalenderstatus wird geladen'

  return (
    <div>
      <PageHeader
        eyebrow="LANDKREIS KELHEIM · AWIDO"
        title="Abfallkalender"
        subtitle="Stadt Riedenburg · Ortsteil Jachenhausen"
        status={statusLabel}
        statusDetail={statusDetail}
        statusMessage={status?.error || apiError ? (status?.error ?? apiError) : undefined}
        actions={(
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <button
              type="button"
              onClick={() => void refreshCalendar()}
              disabled={requestingRefresh}
              style={{ ...pageHeaderButtonStyle('#fff', '#263d52'), opacity: requestingRefresh ? 0.65 : 1 }}
            >
              {requestingRefresh ? 'Lädt …' : 'Aktualisieren'}
            </button>
            <button
              type="button"
              aria-expanded={settingsOpen}
              onClick={() => {
                setDraftIntervalWeeks(status?.refreshIntervalWeeks ?? 1)
                setSettingsOpen(open => !open)
              }}
              style={pageHeaderButtonStyle('transparent', '#fff')}
            >
              Einstellungen
            </button>
          </div>
        )}
      />

      {settingsOpen && (
        <section
          aria-label="Kalendereinstellungen"
          style={{ display: 'flex', alignItems: 'end', flexWrap: 'wrap', gap: 14, marginBottom: 16, padding: '14px 16px', border: '1px solid #cbd5df', borderRadius: 3, background: '#f3f7f7' }}
        >
          <label style={{ display: 'grid', gap: 6, color: '#263d52', fontSize: 13, fontWeight: 700 }}>
            Automatisch neu laden
            <select
              value={draftIntervalWeeks}
              onChange={event => setDraftIntervalWeeks(Number(event.target.value))}
              style={{ minWidth: 190, padding: '8px 10px', border: '1px solid #aebcc7', borderRadius: 3, background: '#fff', color: '#263d52' }}
            >
              {refreshOptions.map(weeks => (
                <option key={weeks} value={weeks}>{weeks === 1 ? 'Jede Woche' : `Alle ${weeks} Wochen`}</option>
              ))}
            </select>
          </label>
          <button type="button" onClick={() => void saveSettings()} disabled={savingSettings} style={pageHeaderButtonStyle('#263d52', '#fff')}>
            {savingSettings ? 'Speichert …' : 'Speichern'}
          </button>
          <button type="button" onClick={() => setSettingsOpen(false)} disabled={savingSettings} style={pageHeaderButtonStyle('#fff', '#263d52')}>
            Schließen
          </button>
        </section>
      )}

      {actionMessage && <p role="status" style={{ margin: '0 0 12px', color: '#425466', fontSize: 13 }}>{actionMessage}</p>}

      <iframe
        key={`${status?.instanceId ?? 'initial'}:${status?.reloadVersion ?? 0}`}
        title="Abfallkalender Stadt Riedenburg, Ortsteil Jachenhausen"
        src={status?.calendarUrl || CALENDAR_URL}
        loading="eager"
        style={{ display: 'block', width: '100%', height: 'calc(100vh - 230px)', minHeight: 640, border: '1px solid #d1d8de', background: '#fff' }}
      />
    </div>
  )
}