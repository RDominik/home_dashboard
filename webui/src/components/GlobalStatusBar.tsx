import { useEffect, useState } from 'react'

type WeatherObservation = {
  temperature: number
  temperatureUnit: string
  windSpeed: number
  windUnit: string
}

type WeatherStatus = { observation?: WeatherObservation }
type EnergyStatus = { ppv?: number | string | null; house_consumption?: number | string | null }
type WastePickup = { date: string; fractions: string[] }
type WasteStatus = { nextPickup?: WastePickup }
type LiveStatus = { weather?: WeatherObservation; energy?: EnergyStatus; pickup?: WastePickup }

const collectionIcons: Record<string, { symbol: string; label: string }> = {
  bio: { symbol: '🍃', label: 'Biotonne' },
  paper: { symbol: '📄', label: 'Papierabfuhr' },
  residual: { symbol: '🗑️', label: 'Restmüll' },
  yellow: { symbol: '🟡', label: 'Gelber Sack' },
}

/**
 * @brief Fetches and decodes one JSON API response for the global status strip.
 * @details Non-success HTTP responses are treated as unavailable data so the caller can keep unrelated live sources visible without masking a failed endpoint as a valid empty payload.
 * @param url Same-origin API URL returning a JSON response.
 * @return Parsed response body with the requested payload type.
 * @throws Error when the server rejects the request or the response is not valid JSON.
 */
async function fetchStatus<T>(url: string): Promise<T> {
  const response = await fetch(url)
  if (!response.ok) throw new Error(`HTTP ${response.status}`)
  return response.json() as Promise<T>
}

/**
 * @brief Formats a numeric or numeric-string power measurement in watts.
 * @details The inverter currently returns MQTT-backed values without a formal JSON number contract, so both JSON numbers and numeric strings are accepted while missing, null, and non-finite values remain a neutral dash.
 * @param value Raw inverter value from the summary endpoint.
 * @return Locale-formatted whole watts or a neutral placeholder when no valid reading exists.
 */
function formatPower(value?: number | string | null): string {
  if (value === undefined || value === null || value === '') return '—'
  const watts = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(watts) ? `${Math.round(watts).toLocaleString('de-DE')} W` : '—'
}

/**
 * @brief Formats the concrete date of the next waste collection for compact display.
 * @details The API uses a date-only value, so its year, month, and day are parsed as local date components rather than as UTC midnight, avoiding a one-day shift in western time zones.
 * @param value ISO local calendar date in YYYY-MM-DD format.
 * @return German day and month, with a two-digit year when the date falls in another year, or a neutral placeholder for invalid input.
 */
function formatPickupDate(value?: string): string {
  if (!value) return '—'
  const [year, month, day] = value.split('-').map(Number)
  const date = new Date(year, month - 1, day)
  if (!Number.isFinite(date.getTime())) return '—'
  const options: Intl.DateTimeFormatOptions = { day: '2-digit', month: '2-digit' }
  if (date.getFullYear() !== new Date().getFullYear()) options.year = '2-digit'
  return date.toLocaleDateString('de-DE', options)
}

/**
 * @brief Displays live weather, photovoltaic, household-load, and next-pickup information across every routed page.
 * @details The component polls existing same-origin APIs independently every 30 seconds, keeps source failures isolated, and renders a neutral placeholder for unavailable observations. Waste fractions are rendered as labeled icons only, including every fraction sharing the nearest pickup date.
 * @return A responsive application-wide status footer that stays visible while page content scrolls.
 */
export default function GlobalStatusBar() {
  const [status, setStatus] = useState<LiveStatus>({})

  useEffect(() => {
    let active = true
    let requestPending = false

    const loadStatus = async () => {
      if (requestPending) return
      requestPending = true
      const [weatherResult, energyResult, wasteResult] = await Promise.allSettled([
        fetchStatus<WeatherStatus>('/api/weather/status'),
        fetchStatus<EnergyStatus>('/api/inverter/summary'),
        fetchStatus<WasteStatus>('/api/waste-calendar/status'),
      ])
      requestPending = false
      if (!active) return
      setStatus({
        weather: weatherResult.status === 'fulfilled' ? weatherResult.value.observation : undefined,
        energy: energyResult.status === 'fulfilled' ? energyResult.value : undefined,
        pickup: wasteResult.status === 'fulfilled' ? wasteResult.value.nextPickup : undefined,
      })
    }

    void loadStatus()
    const timer = window.setInterval(() => void loadStatus(), 30_000)
    return () => {
      active = false
      window.clearInterval(timer)
    }
  }, [])

  const weatherDetail = status.weather
    ? `${status.weather.windSpeed.toLocaleString('de-DE', { maximumFractionDigits: 1 })} ${status.weather.windUnit}`
    : ''
  const pickupDate = formatPickupDate(status.pickup?.date)
  const pickupFractions = (status.pickup?.fractions ?? []).flatMap(fraction => {
    const icon = collectionIcons[fraction]
    return icon ? [{ fraction, ...icon }] : []
  })

  return (
    <footer className="global-status-bar" role="region" aria-label="Aktuelle Hausinformationen">
      <div className="global-status-item" aria-label="Wetterstation">
        <span className="global-status-icon" aria-hidden="true">🌡️</span>
        <div className="global-status-copy">
          <span className="global-status-label">Wetter</span>
          <strong className="global-status-value">
            {status.weather
              ? `${status.weather.temperature.toLocaleString('de-DE', { maximumFractionDigits: 1 })} ${status.weather.temperatureUnit}`
              : '—'}
          </strong>
          {weatherDetail && <span className="global-status-detail">Wind {weatherDetail}</span>}
        </div>
      </div>

      <div className="global-status-item" aria-label="Aktuelle PV-Leistung">
        <span className="global-status-icon" aria-hidden="true">☀️</span>
        <div className="global-status-copy">
          <span className="global-status-label">PV</span>
          <strong className="global-status-value">{formatPower(status.energy?.ppv)}</strong>
        </div>
      </div>

      <div className="global-status-item" aria-label="Aktueller Hausverbrauch">
        <span className="global-status-icon" aria-hidden="true">⌂</span>
        <div className="global-status-copy">
          <span className="global-status-label">Hausverbrauch</span>
          <strong className="global-status-value">{formatPower(status.energy?.house_consumption)}</strong>
        </div>
      </div>

      <div className="global-status-item global-status-pickup" aria-label={`Nächste Müllabfuhr: ${pickupDate}`}>
        <div className="global-status-copy">
          <span className="global-status-label">Müllabfuhr</span>
          <div className="global-pickup-line">
            <strong className="global-status-value">{pickupDate}</strong>
            <span className="global-pickup-icons" aria-label={pickupFractions.map(item => item.label).join(', ') || 'Keine Abfuhrdaten'}>
              {pickupFractions.map(item => (
                <span key={item.fraction} role="img" aria-label={item.label} title={item.label}>{item.symbol}</span>
              ))}
              {pickupFractions.length === 0 && <span aria-hidden="true">—</span>}
            </span>
          </div>
        </div>
      </div>
    </footer>
  )
}