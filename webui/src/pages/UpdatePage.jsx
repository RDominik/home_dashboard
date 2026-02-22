import React, { useState, useEffect } from 'react'

const API = '/api/wallbox'

export default function UpdatePage() {
  const [status, setStatus] = useState(null)
  const [sending, setSending] = useState(false)
  const [feedback, setFeedback] = useState(null)
  const [updating, setUpdating] = useState(false)
  const [updateLog, setUpdateLog] = useState(null)





  const frcLabels = { 0: 'Neutral', 1: 'Aus (Idle)', 2: 'Laden erzwingen' }
  const carLabels = { 1: 'Kein Fahrzeug', 2: 'Laden', 3: 'Warten', 4: 'Fertig' }

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
      <h1 style={{ marginBottom: 5 }}>Update App</h1>
      <p style={{ color: '#6b7280', marginTop: 0, marginBottom: 24 }}>
        
      </p>

      {/* Feedback */}
      {feedback && (
        <div style={{
          padding: '10px 16px',
          borderRadius: 8,
          marginBottom: 500,
          background: feedback.type === 'success' ? '#d1fae5' : '#e3e2fe',
          color: feedback.type === 'success' ? '#065f46' : '#991b1b',
          fontWeight: 500,
        }}>
          {feedback.msg}
        </div>
      )}






      {/* System Update */}
      <div style={{ ...cardStyle, marginTop: 650, borderLeft: '4px solid #f59e0b' }}>
        <h3 style={{ marginTop: 0, color: '#374151' }}>🚀 System Update</h3>
        <p style={{ color: '#6b7280', fontSize: 14, margin: '0 0 12px' }}>
          Git Pull + Docker Container neu bauen und starten
        </p>
        <button
          style={btnStyle(updating ? '#9ca3af' : '#f59e0b')}
          disabled={updating}
          onClick={async () => {
            setUpdating(true)
            setUpdateLog(null)
            try {
              const r = await fetch('/api/system/update', { method: 'POST' })
              const data = await r.json()
              setUpdateLog(data)
            } catch (err) {
              setUpdateLog({ ok: false, results: [{ step: 'fetch', ok: false, stderr: err.message }] })
            } finally {
              setUpdating(false)
            }
          }}
        >
          {updating ? '⏳ Update läuft…' : '🔄 Jetzt updaten'}
        </button>

        {updateLog && (
          <div style={{
            marginTop: 12,
            padding: 12,
            borderRadius: 8,
            background: updateLog.ok ? '#d1fae5' : '#fee2e2',
            fontSize: 13,
          }}>
            <div style={{ fontWeight: 600, marginBottom: 6, color: updateLog.ok ? '#065f46' : '#991b1b' }}>
              {updateLog.ok ? '✅ Update erfolgreich!' : '❌ Update fehlgeschlagen'}
            </div>
            {updateLog.results?.map((r, i) => (
              <div key={i} style={{ marginBottom: 6 }}>
                <span style={{ fontWeight: 600 }}>{r.step}:</span>{' '}
                <span style={{ color: r.ok ? '#065f46' : '#991b1b' }}>{r.ok ? '✓' : '✗'}</span>
                {r.stdout && <pre style={{ margin: '4px 0', fontSize: 12, whiteSpace: 'pre-wrap', color: '#374151' }}>{r.stdout}</pre>}
                {r.stderr && <pre style={{ margin: '4px 0', fontSize: 12, whiteSpace: 'pre-wrap', color: '#991b1b' }}>{r.stderr}</pre>}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function StatusItem({ label, value }) {
  return (
    <div style={{
      background: '#112233',
      borderRadius: 8,
      padding: '10px 14px',
      textAlign: 'center',
    }}>
      <div style={{ fontSize: 12, color: '#6b7280', marginBottom: 4 }}>{label}</div>
      <div style={{ fontSize: 18, fontWeight: 700, color: '#111827' }}>{value}</div>
    </div>
  )
}
