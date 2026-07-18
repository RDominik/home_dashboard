import React, { useMemo, useState } from 'react'

function buildSystemUpdateUrls() {
  const protocol = window.location.protocol
  const host = window.location.hostname
  const configuredBase = import.meta.env.VITE_SYSTEM_UPDATE_BASE_URL?.trim()
  const configuredHostIp = import.meta.env.VITE_SYSTEM_UPDATE_HOST_IP?.trim()
  const candidates = [
    configuredBase,
    `${protocol}//${host}:8090`,
    configuredHostIp ? `${protocol}//${configuredHostIp}:8090` : null,
    `${protocol}//192.168.188.97:8090`,
    `${protocol}//localhost:8090`,
    `${protocol}//127.0.0.1:8090`,
    configuredHostIp ? `http://${configuredHostIp}:8090` : null,
    'http://192.168.188.97:8090',
    'http://localhost:8090',
    'http://127.0.0.1:8090',
  ].filter(Boolean)

  const unique = []
  for (const base of candidates) {
    const normalized = String(base).replace(/\/$/, '')
    if (!unique.includes(normalized)) {
      unique.push(normalized)
    }
  }

  return unique.map((base) => `${base}/api/system/update`)
}

export default function UpdatePage() {
  const [updating, setUpdating] = useState(false)
  const [updateLog, setUpdateLog] = useState(null)

  const updateUrls = useMemo(() => buildSystemUpdateUrls(), [])
  const updateUrl = updateUrls[0]

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
    cursor: updating ? 'wait' : 'pointer',
    opacity: updating ? 0.6 : 1,
    transition: 'opacity 0.2s',
  })

  return (
    <div style={{ maxWidth: 700, margin: '0 auto' }}>
      <h1 style={{ marginBottom: 5 }}>Update App</h1>
      <p style={{ color: '#6b7280', marginTop: 0, marginBottom: 24 }}>
        Nutzt den externen Host-Service auf Port 8090.
      </p>

      {/* System Update */}
      <div style={{ ...cardStyle, borderLeft: '4px solid #f59e0b' }}>
        <h3 style={{ marginTop: 0, color: '#374151' }}>🚀 System Update</h3>
        <p style={{ color: '#6b7280', fontSize: 14, margin: '0 0 12px' }}>
          Git Pull + Docker Container neu bauen und starten
        </p>
        <p style={{ color: '#6b7280', fontSize: 12, margin: '0 0 12px' }}>
          Endpoint: {updateUrl}
        </p>
        <button
          style={btnStyle(updating ? '#9ca3af' : '#f59e0b')}
          disabled={updating}
          onClick={async () => {
            setUpdating(true)
            setUpdateLog(null)
            let lastFetchError = null
            try {
              for (const candidateUrl of updateUrls) {
                try {
                  const r = await fetch(candidateUrl, { method: 'POST' })
                  const data = await r.json()
                  setUpdateLog(data)
                  return
                } catch (err) {
                  lastFetchError = err
                }
              }

              const msg = lastFetchError instanceof Error ? lastFetchError.message : String(lastFetchError)
              setUpdateLog({
                ok: false,
                results: [{
                  step: 'fetch',
                  ok: false,
                  stderr:
                    `Service nicht erreichbar. Versuchte URLs: ${updateUrls.join(', ')}. Letzter Fehler: ${msg}. ` +
                    'Bitte starten: cd /home/dominik/Repository/webgui/systemUpdate && go run . --repo-dir /home/dominik/Repository/webgui --addr :8090',
                }],
              })
            } catch (err) {
              const msg = err instanceof Error ? err.message : String(err)
              setUpdateLog({
                ok: false,
                results: [{
                  step: 'fetch',
                  ok: false,
                  stderr: `Service nicht erreichbar (${updateUrl}). ${msg}.`,
                }],
              })
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
