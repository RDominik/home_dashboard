import React, { useEffect, useMemo, useState } from 'react'
import './Heating.css'

const TABS = [
  { key: 'kessel', label: 'Kessel' },
  { key: 'puffer', label: 'Puffer' },
  { key: 'hk', label: 'HK' },
  { key: 'efilter', label: 'E.Filter' },
  { key: 'sys', label: 'Sys' },
]

const DEFAULT_METRICS = {
  boiler_temp: 0,
  boiler_pressure: 0,
  buffer_top: 0,
  buffer_bottom: 0,
  return_temp: 0,
  outside_temp: 0,
  feed_rate: 0,
  burner_status: 'Standby',
}

export default function Heating() {
  const [d, setD] = useState(null)
  const [activeTab, setActiveTab] = useState('kessel')
  const [now, setNow] = useState(new Date())
  const metrics = useMemo(() => mapEtaTreeToMetrics(d), [d])

  useEffect(() => {
    const base = window.location.origin.replace(':8080', ':8081')
    const tick = () => fetch(base + '/api/heating/summary').then((r) => r.json()).then(setD).catch(() => {})
    tick()
    const t = setInterval(tick, 5000)
    return () => clearInterval(t)
  }, [])

  useEffect(() => {
    const t = setInterval(() => setNow(new Date()), 1000)
    return () => clearInterval(t)
  }, [])

  const statusText = useMemo(() => {
    if (!metrics) return 'Bereit'
    return /bereit|ein|on/i.test(String(metrics.burner_status ?? '')) ? 'Bereit' : 'Standby'
  }, [metrics])

  return (
    <div className="eta-wrap">
      <div className="eta-screen">
        <header className="eta-top">
          <div className="eta-brand">ETA</div>
          <nav className="eta-tabs" aria-label="Heating subpages">
            {TABS.map((tab) => (
              <button
                type="button"
                key={tab.key}
                className={activeTab === tab.key ? 'eta-tab eta-tab-active' : 'eta-tab'}
                onClick={() => setActiveTab(tab.key)}
              >
                {tab.label}
              </button>
            ))}
          </nav>
          <div className="eta-top-icons">
            <div className="eta-icon">Play</div>
            <div className="eta-icon">View</div>
            <div className="eta-icon">?</div>
          </div>
        </header>

        <div className="eta-main">
          <section className="eta-stage">
            <div className="eta-status-box">{statusText}</div>
            {activeTab === 'kessel' && <KesselSubpage d={metrics} />}
            {activeTab === 'puffer' && <PufferSubpage d={metrics} />}
            {activeTab === 'hk' && <HkSubpage d={metrics} />}
            {activeTab === 'efilter' && <EFilterSubpage d={metrics} />}
            {activeTab === 'sys' && <SysSubpage />}
          </section>

          <aside className="eta-side">
            {activeTab === 'kessel' && <RightActions items={['Entaschen', 'Messung', 'Einstellungen']} />}
            {activeTab === 'puffer' && <RightActions items={['Warmwasser sofort laden', 'Einstellungen']} />}
            {activeTab === 'hk' && <RightActions items={['Zeitautomatik', 'Einstellungen']} />}
            {activeTab === 'efilter' && <RightActions items={['Einstellungen']} />}
            {activeTab === 'sys' && <RightActions items={['Einstellungen']} />}
          </aside>
        </div>

        <footer className="eta-footer">
          <div className="eta-foot-item">Aussen: {fmt(metrics?.outside_temp)} C</div>
          <div className="eta-foot-item eta-foot-time">{new Intl.DateTimeFormat('de-DE', { dateStyle: 'medium', timeStyle: 'medium' }).format(now)}</div>
        </footer>
      </div>
    </div>
  )
}

function mapEtaTreeToMetrics(tree) {
  if (!tree) return DEFAULT_METRICS

  const boilerTemp = pickNumber(
    tree?.eta?.Eingänge?.Kessel?.StrValue,
    tree?.eta?.Kessel?.Kessel?.Kessel_Soll?.Angeforderte_Temperatur?.StrValue,
    tree?.eta?.Kessel?.Rücklaufanhebung?.Rücklauf?.Rücklaufmischer?.Ist_Temperatur?.StrValue,
    DEFAULT_METRICS.boiler_temp,
  )

  const returnTemp = pickNumber(
    tree?.eta?.Eingänge?.Rücklauf?.StrValue,
    DEFAULT_METRICS.return_temp,
  )

  const bufferTop = pickNumber(
    tree?.eta?.Eingänge?.Puffer_oben?.StrValue,
    tree?.eta?.Puffer?.Puffer_oben?.StrValue,
    tree?.eta?.Puffer?.Kaskade?.Regler_Kaskade?.Istwert?.StrValue,
    boilerTemp,
    DEFAULT_METRICS.buffer_top,
  )

  const bufferBottom = pickNumber(
    tree?.eta?.Eingänge?.Puffer_unten?.StrValue,
    tree?.eta?.Puffer?.Puffer_unten?.StrValue,
    tree?.eta?.Puffer?.Kaskade?.Regler_Kaskade?.Sollwert?.StrValue,
    returnTemp,
    DEFAULT_METRICS.buffer_bottom,
  )

  return {
    boiler_temp: boilerTemp,
    boiler_pressure: pickNumber(
      tree?.eta?.Eingänge?.Kesseldruck?.Kesseldruck?.StrValue,
      tree?.eta?.Kessel?.Kesseldruck?.StrValue,
      DEFAULT_METRICS.boiler_pressure,
    ),
    buffer_top: bufferTop,
    buffer_bottom: bufferBottom,
    return_temp: returnTemp,
    outside_temp: pickNumber(
      tree?.eta?.Eingänge?.Außentemperatur?.StrValue,
      tree?.eta?.Außentemperatur?.StrValue,
      DEFAULT_METRICS.outside_temp,
    ),
    feed_rate: pickNumber(
      tree?.eta?.Austragung?.Stoker_Einheit?.Taktrate?.StrValue,
      tree?.eta?.Ausgänge?.Austragung?.Taktrate?.StrValue,
      DEFAULT_METRICS.feed_rate,
    ),
    burner_status: pickString(
      tree?.eta?.Kessel?.StrValue,
      tree?.eta?.Kessel?.Entaschung?.Rost_Zustand?.Rost?.Zustand?.StrValue,
      DEFAULT_METRICS.burner_status,
    ),
  }
}

function pickString(...values) {
  for (const value of values) {
    if (typeof value === 'string' && value.trim() !== '') {
      return value
    }
  }
  return null
}

function pickNumber(...values) {
  for (const value of values) {
    if (value == null) continue
    const normalized = String(value).replace(/\./g, '').replace(',', '.').trim()
    const parsed = Number.parseFloat(normalized)
    if (Number.isFinite(parsed)) {
      return parsed
    }
  }
  return null
}

function MetricPill({ label, value, unit }) {
  return (
    <div className="eta-pill">
      <span className="eta-pill-label">{label}</span>
      <strong>
        {value}
        {unit ? ` ${unit}` : ''}
      </strong>
    </div>
  )
}

function RightActions({ items }) {
  return (
    <div className="eta-actions">
      {items.map((item) => (
        <button type="button" key={item} className="eta-action-btn">
          {item}
        </button>
      ))}
    </div>
  )
}

function KesselSubpage({ d }) {
  const pressureValue = d?.boiler_pressure ?? (typeof d?.boiler_temp === 'number' ? d.boiler_temp / 38 : null)
  const pressureText = pressureValue == null ? '' : `${fmt(pressureValue)} bar`

  return (
    <div className="eta-view eta-grid-kessel">
      <div className="eta-machine-block eta-kessel-block">
        <div className="eta-kessel-canvas">
          <img className="eta-kessel-image" src="/produktbild-eta-hackgutkessel-ehack.png" alt="Heizkessel" />
          <div className="eta-kessel-label eta-kessel-label-top">{fmt(d?.boiler_temp)} C</div>
          <div className="eta-kessel-label eta-kessel-label-mid">{pressureText}</div>
          <div className="eta-kessel-label eta-kessel-label-bottom">{fmt(d?.return_temp)} C</div>
          <div className="eta-kessel-line eta-kessel-line-top-h" />
          <div className="eta-kessel-line eta-kessel-line-top-v" />
          <div className="eta-kessel-dot eta-kessel-dot-top" />
          <div className="eta-kessel-line eta-kessel-line-bottom" />
          <div className="eta-kessel-dot eta-kessel-dot-bottom" />
        </div>
      </div>
      <div className="eta-metrics-col">
        <MetricPill label="Kessel" value={fmt(d?.boiler_temp)} unit="C" />
        <MetricPill label="Druck" value={pressureValue == null ? '' : fmt(pressureValue)} unit={pressureValue == null ? '' : 'bar'} />
        <MetricPill label="Rücklauf" value={fmt(d?.return_temp)} unit="C" />
        <MetricPill label="Aussen" value={fmt(d?.outside_temp)} unit="C" />
      </div>
    </div>
  )
}

function PufferSubpage({ d }) {
  const topValue = `${fmt(d?.buffer_top)} C`
  const bottomValue = `${fmt(d?.buffer_bottom)} C`
  const spread =
    typeof d?.buffer_top === 'number' && typeof d?.buffer_bottom === 'number'
      ? `${fmt(d.buffer_top - d.buffer_bottom)} C`
      : '-- C'

  return (
    <div className="eta-view eta-grid-puffer">
      <div className="eta-machine-block eta-puffer-block">
        <div className="eta-puffer-canvas">
          <img className="eta-puffer-image" src="/Puffer.png" alt="Puffer" />
          <div className="eta-puffer-label eta-puffer-label-top">{topValue}</div>
          <div className="eta-puffer-label eta-puffer-label-bottom">{bottomValue}</div>
          <div className="eta-puffer-label eta-puffer-label-side">{spread}</div>
        </div>
      </div>
      <div className="eta-metrics-col">
        <MetricPill label="Puffer oben" value={fmt(d?.buffer_top)} unit="C" />
        <MetricPill label="Puffer unten" value={fmt(d?.buffer_bottom)} unit="C" />
        <MetricPill label="Delta" value={spread} />
      </div>
    </div>
  )
}

function HkSubpage({ d }) {
  return (
    <div className="eta-view eta-grid-hk">
      <div>
        <div className="eta-hk-radiator" />
        <div className="eta-hk-schedule">
          <div className="eta-hk-track" />
          <div className="eta-hk-window eta-hk-window-a" />
          <div className="eta-hk-window eta-hk-window-b" />
        </div>
      </div>
      <div className="eta-metrics-col">
        <MetricPill label="Aussen" value={fmt(d?.outside_temp)} unit="C" />
        <MetricPill label="Modus" value="Zeitautomatik" />
      </div>
    </div>
  )
}

function EFilterSubpage({ d }) {
  return (
    <div className="eta-view eta-grid-efilter">
      <div className="eta-machine-block">
        <div className="eta-machine-art eta-filter" />
      </div>
      <div className="eta-metrics-col">
        <MetricPill label="Spannung" value={fmt((d?.boiler_temp || 0) / 2400)} unit="kV" />
        <MetricPill label="Strom" value={fmt((d?.feed_rate || 0) / 9000)} unit="mA" />
      </div>
    </div>
  )
}

function SysSubpage() {
  return (
    <div className="eta-view eta-grid-sys">
      <div className="eta-house-outline" />
      <div className="eta-metrics-col">
        <MetricPill label="System" value="Bereit" />
      </div>
    </div>
  )
}

function fmt(v) {
  if (v == null) return '--'
  return new Intl.NumberFormat('de-DE', { maximumFractionDigits: 2 }).format(v)
}
