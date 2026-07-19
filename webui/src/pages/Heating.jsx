import React, { useEffect, useMemo, useState } from 'react'
import './Heating.css'

const TABS = [
  { key: 'kessel', label: 'Kessel' },
  { key: 'puffer', label: 'Puffer' },
  { key: 'hk', label: 'HK' },
  { key: 'efilter', label: 'E.Filter' },
  { key: 'sys', label: 'Sys' },
]

const HK_TABS = [
  { key: 'heizen', label: 'Heizen' },
  { key: 'absenken', label: 'Absenken' },
  { key: 'zeitautomatik', label: 'Zeitautomatik' },
  { key: 'heizzeiten', label: 'Heizzeiten' },
]

const DEFAULT_METRICS = {
  boiler_temp: 0,
  boiler_pressure: 0,
  buffer_top: 0,
  buffer_bottom: 0,
  warmwater_storage: 0,
  return_temp: 0,
  outside_temp: 0,
  feed_rate: 0,
  heating_status: 'Bereit',
  heating_status_extra: 'Heizgrenze erreicht',
  burner_status: 'Standby',
}

const PANEL_STATUS = {
  kessel: { main: 'Kessel aktiv', sub: 'Dummywert: Kesselstatus' },
  puffer: { main: 'Puffer geladen', sub: 'Dummywert: Pufferspeicher' },
  efilter: { main: 'E.Filter bereit', sub: 'Dummywert: Filterzustand' },
  sys: { main: 'System ok', sub: 'Dummywert: Systemstatus' },
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

function pickByURI(objects, ...uris) {
  if (!Array.isArray(objects)) return undefined
  for (const uri of uris) {
    const match = objects.find((o) => o.URI === uri)
    if (match?.StrValue != null && match.StrValue !== '') return match.StrValue
  }
  return undefined
}

function mapEtaTreeToMetrics(tree) {
  if (!tree) return DEFAULT_METRICS

  const objs = tree?.Variable?.Objects

  return {
    boiler_temp: pickNumber(
      pickByURI(objs, '24/10561/0/0/12161', '24/10561/0/11109'),
      DEFAULT_METRICS.boiler_temp,
    ),
    boiler_pressure: pickNumber(
      pickByURI(objs, '24/10561/0/0/12180'),
      DEFAULT_METRICS.boiler_pressure,
    ),
    buffer_top: pickNumber(
      pickByURI(objs, '120/10251/0/0/12242', '120/10251/0/11153'),
      DEFAULT_METRICS.buffer_top,
    ),
    buffer_bottom: pickNumber(
      pickByURI(objs, '120/10251/0/0/12244', '120/10251/0/11155'),
      DEFAULT_METRICS.buffer_bottom,
    ),
    warmwater_storage: pickNumber(
      pickByURI(objs, '120/10251/0/0/12271', '120/10251/0/11129'),
      DEFAULT_METRICS.warmwater_storage,
    ),
    return_temp: pickNumber(
      pickByURI(objs, '24/10561/0/0/12220', '24/10561/0/11160'),
      DEFAULT_METRICS.return_temp,
    ),
    outside_temp: pickNumber(
      pickByURI(objs, '120/10101/0/0/12197', '120/10241/0/0/12197', '120/10241/0/11127'),
      DEFAULT_METRICS.outside_temp,
    ),
    feed_rate: pickNumber(
      DEFAULT_METRICS.feed_rate,
    ),
    heating_status: pickString(
      pickByURI(objs, '120/10101/0/0/19404'),
      DEFAULT_METRICS.heating_status,
    ),
    heating_status_extra: pickString(
      pickByURI(objs, '120/10101/0/0/19391'),
      DEFAULT_METRICS.heating_status_extra,
    ),
    burner_status: pickString(
      pickByURI(objs, '24/10561/0/0/12000'),
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
  const status = PANEL_STATUS.kessel
  const pressureValue = d?.boiler_pressure ?? (typeof d?.boiler_temp === 'number' ? d.boiler_temp / 38 : null)
  const pressureText = pressureValue == null ? '' : `${fmt(pressureValue)} bar`

  return (
    <div className="eta-view eta-grid-kessel">
      <TabStatusBox main={status.main} sub={status.sub} />
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
  const status = PANEL_STATUS.puffer
  const topValue = `${fmt(d?.buffer_top)} C`
  const bottomValue = `${fmt(d?.buffer_bottom)} C`
  const spread =
    typeof d?.buffer_top === 'number' && typeof d?.buffer_bottom === 'number'
      ? `${fmt(d.buffer_top - d.buffer_bottom)} C`
      : '-- C'

  return (
    <div className="eta-view eta-grid-puffer">
      <TabStatusBox main={status.main} sub={status.sub} />
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
        <MetricPill label="Warmwasserspeicher" value={fmt(d?.warmwater_storage)} unit="C" />
      </div>
    </div>
  )
}

function HkSubpage({ d }) {
  const [activeHkTab, setActiveHkTab] = useState('heizen')
  const [heizenPermanent, setHeizenPermanent] = useState(false)
  const [expandedDay, setExpandedDay] = useState('mo')
  const statusText = pickString(d?.heating_status, d?.burner_status, 'Bereit') ?? 'Bereit'
  const statusSubtext = pickString(d?.heating_status_extra, '')
  const [heizzzeiten, setHeizzzeiten] = useState({
    mo: [{ von: '06:00', bis: '09:00' }, { von: '16:00', bis: '22:00' }, { von: '', bis: '' }],
    di: [{ von: '06:00', bis: '09:00' }, { von: '16:00', bis: '22:00' }, { von: '', bis: '' }],
    mi: [{ von: '06:00', bis: '09:00' }, { von: '16:00', bis: '22:00' }, { von: '', bis: '' }],
    do: [{ von: '06:00', bis: '09:00' }, { von: '16:00', bis: '22:00' }, { von: '', bis: '' }],
    fr: [{ von: '06:00', bis: '09:00' }, { von: '16:00', bis: '22:00' }, { von: '', bis: '' }],
    sa: [{ von: '07:00', bis: '22:00' }, { von: '', bis: '' }, { von: '', bis: '' }],
    so: [{ von: '07:00', bis: '21:00' }, { von: '', bis: '' }, { von: '', bis: '' }],
  })

  const days = [
    { key: 'mo', label: 'Montag' },
    { key: 'di', label: 'Dienstag' },
    { key: 'mi', label: 'Mittwoch' },
    { key: 'do', label: 'Donnerstag' },
    { key: 'fr', label: 'Freitag' },
    { key: 'sa', label: 'Samstag' },
    { key: 'so', label: 'Sonntag' },
  ]

  const updateHeizzzeit = (day, idx, field, val) => {
    setHeizzzeiten((prev) => ({
      ...prev,
      [day]: prev[day].map((hz, i) => i === idx ? { ...hz, [field]: val } : hz),
    }))
  }

  const deleteHeizzzeit = (day, idx) => {
    setHeizzzeiten((prev) => ({
      ...prev,
      [day]: prev[day].filter((_, i) => i !== idx),
    }))
  }

  const addHeizzzeit = (day) => {
    setHeizzzeiten((prev) => {
      if (prev[day].length >= 3) {
        return prev
      }

      return {
        ...prev,
        [day]: [...prev[day], { von: '', bis: '' }],
      }
    })
  }

  return (
    <div className="eta-view eta-grid-hk">
      <div>
        <div className="eta-status-box eta-status-box-hk">
          <div className="eta-status-main">{statusText}</div>
          {statusSubtext && <div className="eta-status-sub">{statusSubtext}</div>}
        </div>

        <div className="eta-hk-tabs" role="tablist" aria-label="HK Betriebsarten">
          {HK_TABS.map((tab) => (
            <button
              key={tab.key}
              type="button"
              role="tab"
              aria-selected={activeHkTab === tab.key}
              className={activeHkTab === tab.key ? 'eta-hk-tab eta-hk-tab-active' : 'eta-hk-tab'}
              onClick={() => setActiveHkTab(tab.key)}
            >
              {tab.label}
            </button>
          ))}
        </div>

        <div className="eta-hk-panel" role="tabpanel" aria-label="HK Inhalt">
          <div className="eta-hk-radiator" />

          {activeHkTab === 'heizen' && (
            <div className="eta-hk-heizen-switch-row">
              <span className="eta-hk-heizen-switch-label-left">Bis zur naechsten Absenkzeit</span>
              <button
                type="button"
                role="switch"
                aria-checked={heizenPermanent}
                aria-label="Heizen dauerhaft"
                className={heizenPermanent ? 'eta-hk-switch eta-hk-switch-on' : 'eta-hk-switch'}
                onClick={() => setHeizenPermanent((prev) => !prev)}
              >
                <span className="eta-hk-switch-thumb" />
              </button>
              <span className="eta-hk-heizen-switch-label-right">Dauernd</span>
            </div>
          )}

          {activeHkTab === 'absenken' && (
            <div className="eta-hk-mode-pill">Aktiv: Absenken</div>
          )}

          {activeHkTab === 'zeitautomatik' && (
            <div className="eta-hk-schedule">
              <div className="eta-hk-track" />
              <div className="eta-hk-window eta-hk-window-a" />
              <div className="eta-hk-window eta-hk-window-b" />
            </div>
          )}

          {activeHkTab === 'heizzeiten' && (
            <div className="eta-hk-times">
              {days.map((day) => (
                <div key={day.key} className="eta-hk-day-group">
                  <button
                    type="button"
                    className="eta-hk-day-header"
                    onClick={() => setExpandedDay(expandedDay === day.key ? null : day.key)}
                  >
                    <span className={`eta-hk-day-arrow ${expandedDay === day.key ? 'expanded' : ''}`}>▼</span>
                    <span className="eta-hk-day-label">{day.label}</span>
                  </button>

                  {expandedDay === day.key && (
                    <div className="eta-hk-day-content">
                      {heizzzeiten[day.key].map((hz, idx) => (
                        <div key={idx} className="eta-hk-heizzzeit-slot">
                          <span className="eta-hk-heizzzeit-label">Heizzzeit {idx + 1}</span>
                          <div className="eta-hk-heizzzeit-inputs">
                            <span className="eta-hk-time-label">Von...</span>
                            <input
                              type="time"
                              value={hz.von}
                              onChange={(e) => updateHeizzzeit(day.key, idx, 'von', e.target.value)}
                              className="eta-hk-time-input"
                            />
                            <span className="eta-hk-time-label">Bis...</span>
                            <input
                              type="time"
                              value={hz.bis}
                              onChange={(e) => updateHeizzzeit(day.key, idx, 'bis', e.target.value)}
                              className="eta-hk-time-input"
                            />
                            <button
                              type="button"
                              className="eta-hk-delete-btn"
                              onClick={() => deleteHeizzzeit(day.key, idx)}
                              aria-label="Heizzzeit löschen"
                            >
                              🗑️
                            </button>
                          </div>
                        </div>
                      ))}
                      {heizzzeiten[day.key].length < 3 && (
                        <button
                          type="button"
                          className="eta-hk-add-heizzzeit-btn"
                          onClick={() => addHeizzzeit(day.key)}
                        >
                          + Heizzzeit hinzufügen
                        </button>
                      )}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
      <div className="eta-metrics-col">
        <MetricPill label="Aussen" value={fmt(d?.outside_temp)} unit="C" />
        <MetricPill
          label="Modus"
          value={HK_TABS.find((tab) => tab.key === activeHkTab)?.label ?? 'Heizen'}
        />
      </div>
    </div>
  )
}

function TabStatusBox({ main, sub }) {
  return (
    <div className="eta-status-box">
      <div className="eta-status-main">{main}</div>
      {sub && <div className="eta-status-sub">{sub}</div>}
    </div>
  )
}

function EFilterSubpage({ d }) {
  const status = PANEL_STATUS.efilter

  return (
    <div className="eta-view eta-grid-efilter">
      <TabStatusBox main={status.main} sub={status.sub} />
      <div className="eta-machine-block eta-efilter-block">
        <div className="eta-efilter-canvas">
          <div className="eta-efilter-readouts">
            <MetricPill label="Spannung" value={fmt((d?.boiler_temp || 0) / 2400)} unit="kV" />
            <MetricPill label="Strom" value={fmt((d?.feed_rate || 0) / 9000)} unit="mA" />
          </div>
          <img className="eta-efilter-fire" src="/fire_cropped.png" alt="E.Filter Flamme" />
          <img className="eta-efilter-image" src="/EFilter.png" alt="E.Filter" />
        </div>
      </div>
    </div>
  )
}

function SysSubpage() {
  const status = PANEL_STATUS.sys

  return (
    <div className="eta-view eta-grid-sys">
      <TabStatusBox main={status.main} sub={status.sub} />
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
