import React, { useMemo, useState } from 'react'

export default function Grafana() {
  const [url, setUrl] = useState('http://192.168.188.97:3000/public-dashboards/12b8f580467f4aa7b432928f0a4ec2be?refresh=5s&from=now-6h&to=now&timezone=browser')
  const safeUrl = useMemo(() => {
    try {
      const u = new URL(url)
      return u.toString()
    } catch {
      return null
    }
  }, [url])

  return (
    <div style={{height:'100%', display:'flex', flexDirection:'column'}}>
      <h1>Grafana Dashboard</h1>
      <div style={{display:'flex', gap:8, alignItems:'center', marginBottom:8}}>
        <label htmlFor="gurl" style={{color:'#555'}}>URL:</label>
        <input id="gurl" value={url} onChange={e=>setUrl(e.target.value)} style={{flex:1, padding:'8px 10px', border:'1px solid #ddd', borderRadius:6}} />
        <button onClick={()=>setUrl(url)} style={{padding:'8px 12px'}}>Laden</button>
      </div>
      {!safeUrl && <div style={{background:'#fff3cd', border:'1px solid #ffeeba', color:'#856404', padding:10, borderRadius:6, margin:'8px 0'}}>Ungültige URL</div>}
      <div style={{flex:1, minHeight:400, border:'1px solid #eee', borderRadius:8, overflow:'hidden'}}>
        {safeUrl && (
          <iframe title="Grafana" src={safeUrl} style={{width:'100%', height:'100%', border:0}} allowFullScreen />
        )}
      </div>
      <div style={{marginTop:8, color:'#666', fontSize:12}}>
        Hinweis: Das eingebettete Dashboard muss als Public Dashboard freigegeben sein. Bei Mixed‑Content kann HTTPS für die WebUI oder einen Reverse‑Proxy nötig sein.
      </div>
    </div>
  )
}
