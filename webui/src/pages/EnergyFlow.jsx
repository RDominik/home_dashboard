import React, { useState, useEffect } from 'react';

// Eigene SVG-Icons
const SolarPanelIcon = () => (
  <svg viewBox="0 0 60 60" fill="none">
    {/* Sonne im Hintergrund */}
    <circle cx="30" cy="20" r="8" fill="#fbbf24"/>
    {/* Sonnenstrahlen */}
    <line x1="30" y1="8" x2="30" y2="4" stroke="#fbbf24" strokeWidth="2" strokeLinecap="round"/>
    <line x1="30" y1="32" x2="30" y2="36" stroke="#fbbf24" strokeWidth="2" strokeLinecap="round"/>
    <line x1="18" y1="20" x2="14" y2="20" stroke="#fbbf24" strokeWidth="2" strokeLinecap="round"/>
    <line x1="42" y1="20" x2="46" y2="20" stroke="#fbbf24" strokeWidth="2" strokeLinecap="round"/>
    <line x1="22" y1="12" x2="19" y2="9" stroke="#fbbf24" strokeWidth="2" strokeLinecap="round"/>
    <line x1="38" y1="12" x2="41" y2="9" stroke="#fbbf24" strokeWidth="2" strokeLinecap="round"/>
    <line x1="22" y1="28" x2="19" y2="31" stroke="#fbbf24" strokeWidth="2" strokeLinecap="round"/>
    <line x1="38" y1="28" x2="41" y2="31" stroke="#fbbf24" strokeWidth="2" strokeLinecap="round"/>
    
    {/* Solar Panel (geneigt) */}
    <path d="M 15 35 L 45 35 L 50 50 L 10 50 Z" fill="#1e40af" stroke="white" strokeWidth="2"/>
    
    {/* Grid auf Panel */}
    <line x1="20" y1="35" x2="18" y2="50" stroke="white" strokeWidth="1.5"/>
    <line x1="30" y1="35" x2="30" y2="50" stroke="white" strokeWidth="1.5"/>
    <line x1="40" y1="35" x2="42" y2="50" stroke="white" strokeWidth="1.5"/>
    
    <line x1="15" y1="40" x2="48" y2="40" stroke="white" strokeWidth="1.5"/>
    <line x1="13" y1="45" x2="49" y2="45" stroke="white" strokeWidth="1.5"/>
    
    {/* Pfeil von Sonne zu Panel */}
    <line x1="30" y1="28" x2="30" y2="34" stroke="#fbbf24" strokeWidth="2" strokeLinecap="round"/>
    <polygon points="30,35 28,32 32,32" fill="#fbbf24"/>
  </svg>
);

const BatteryIcon = ({ level = 65 }) => {
  // Farbe basierend auf Ladestand
  const getFillColor = () => {
    if (level <= 20) return '#ef4444'; // Rot bei niedrig
    if (level <= 50) return '#f59e0b'; // Orange bei mittel
    return '#10b981'; // Grün bei gut
  };

  return (
    <svg viewBox="0 0 60 60" fill="none">
      {/* Batterie Körper */}
      <rect 
        x="5" 
        y="15" 
        width="45" 
        height="30" 
        rx="3" 
        stroke="#000000"
        strokeWidth="2.5" 
        fill="#dadadd"
      />
      {/* Batterie Plus-Pol */}
      <rect 
        x="50" 
        y="25" 
        width="5"
        height="10" 
        rx="1" 
        fill="#0a05ff"
      />
      
      {/* Füllstand mit dynamischer Farbe */}
      <rect 
        x="8" 
        y="18" 
        width={Math.max(0, 39 * (level / 100))} 
        height="24" 
        rx="2" 
        fill={getFillColor()}
      />
      
      {/* Prozentanzeige */}
      <text 
        x="27" 
        y="35" 
        textAnchor="middle" 
        fill="#a7a7a7"
        fontSize="12" 
        fontWeight="bold"
      >
        {Math.round(level)}%
      </text>
    </svg>
  );
};

const HomeIcon = () => (
  <svg viewBox="0 0 60 60" fill="none">
    {/* Dach */}
    <polygon points="30,5 5,28 12,28 12,52 48,52 48,28 55,28" fill="#757575" stroke="white" strokeWidth="2" strokeLinejoin="round"/>
    {/* Hauswand */}
    <rect x="14" y="28" width="32" height="24" fill="#757575" stroke="white" strokeWidth="1.5"/>
    {/* Tür */}
    <rect x="24" y="36" width="12" height="16" rx="1" fill="#bebac2" stroke="white" strokeWidth="1.5"/>
    {/* Schornstein */}
    <rect x="38" y="10" width="6" height="14" fill="#686768" stroke="white" strokeWidth="1.5"/>
  </svg>
);

const CarIcon = () => (
  <svg viewBox="0 0 225 225" fill="none">
    <path
      d="M57.37 175.62C53.17,177.37 51,177.37 51,175.62C51,174.87 49.6,173.97 47.88,173.63C46.17,173.28 42.97,171.2 40.77,169L36.76 165H27.28C17.83,165 17.79,164.99 14.4,161.6C10.49,157.69 10.32,156.21 12.58,145.21C14.51,135.84 18.25,128.63 22.57,125.96C27.44,122.94 36.1,120.23 48.27,117.9C58.16,116.01 60.02,115.29 67.36,110.51C79.03,102.91 90.77,97.24 99.73,94.88C106.07,93.2 110.9,92.79 126,92.66C142.57,92.52 145.23,92.73 151.51,94.71C159.88,97.34 166.21,101.36 174.99,109.63C180.6,114.9 182.33,115.94 187.5,117.07C191.74,118.01 194.31,119.25 196.25,121.3C198.99,124.2 199,124.27 199,138.61C199,146.68 198.6,153 198.08,153C194.26,153 194,152.05 194,138.42C194,123.19 193.79,122.79 185.03,121.39C181.25,120.78 179.28,119.45 172.01,112.59C163.28,104.33 155.96,99.92 147.89,98.06C145.47,97.5 136.15,97.04 127.17,97.02C113.91,97 109.32,97.39 102.79,99.05C93.22,101.5 81.72,107.09 69.86,115.06C61.65,120.57 60.57,120.99 48.36,123.48C32.75,126.67 24.18,130.36 21.2,135.18C16.88,142.16 14.22,158.26 17.08,160.07C17.86,160.57 22.21,160.98 26.75,160.98L35 161V156.95C35,150.11 39.06,144.54 46.23,141.55C51.22,139.46 55.54,139.6 60.49,142C66.25,144.78 69.27,149.24 69.81,155.71L70.25 161H140.74L141.27 157C143.34,141.63 160.52,134.67 171.37,144.81C174.92,148.13 177,152.81 177,157.47V161H190.5C191.81,161 193.02,161.01 194.15,161.02C200,161.07 203.39,161.11 205.36,159.44C208.08,157.14 208.06,151.6 208.01,138.39C208.01,136.56 208,134.57 208,132.43C208,117.41 207.6,107.12 206.96,105.93C206.08,104.27 204.82,104 198.03,104C183.56,104 182,102.08 182,84.25C182,76.37 181.6,72 180.8,71.2C179.96,70.36 174.31,70 161.8,70H144V72.3C144,75.74 142.11,77 136.95,77C133.5,77 131.82,77.54 130.41,79.1C127.02,82.85 123.45,84 115.25,84C106.86,84 105,83.15 105,79.3C105,77.19 104.56,77 99.7,77C94.88,77 92,75.82 92,73.85C92,73.43 94.81,72.96 98.25,72.8L104.5 72.5L104.8 67.75L105.11 63H98.55C92.72,63 92,62.79 92,61.05C92,59.35 92.82,59.06 98.24,58.8C104.32,58.51 104.5,58.42 105,55.5L105.52 52.5L115.42 52.21C125.06,51.93 125.42,52 128.95,54.84C131.39,56.8 134.28,57.99 137.79,58.47C142.68,59.14 143,59.36 143,62.03V64.88L161.92 65.19C180.37,65.49 180.91,65.56 183.42,67.92C185.93,70.28 186,70.7 186,82.98C186,99.02 186.08,99.14 196.59,99.69C203.21,100.04 207.16,100.11 209.52,102.09C213,105.01 213,112.09 213,130.36V132.8C213,159.62 212.55,162.57 208.06,164.97C206.98,165.55 199.44,166 190.92,166H175.7L172.3 169.5C167.6,174.34 163.31,176.36 157.84,176.29C152.58,176.23 145.86,172.96 143.47,169.31L141.95 167H67.7L64.19 170.62C62.25,172.61 59.18,174.86 57.37,175.62Z
      M46.37 168.08C51.29,171.09 57.22,170.63 61.45,166.91C65.39,163.45 67.62,157.6 66.57,153.43C66.19,151.92 64.38,149.18 62.54,147.34C59.58,144.38 58.59,144 53.82,144C45.09,144 40.09,148.54 40.03,156.53C40,162.21 41.63,165.19 46.37,168.08Z
      M155.5 169.04C161.07,171.36 168.97,168.4 171.72,162.97C173.42,159.61 173.32,153.52 171.51,150.01C169.23,145.62 166.21,144 160.26,144C155.63,144 154.5,144.43 151.24,147.42C147.79,150.58 147.5,151.27 147.5,156.36C147.5,161.11 147.91,162.3 150.5,165.04C152.15,166.78 154.4,168.58 155.5,169.04Z
      M111 55V78H117.58C123.62,78 124.51,77.71 128.43,74.5C131.58,71.92 133.67,71 136.35,71H140V66.5C140,62.02 139.98,62 136.5,62C133.91,62 132,61.09 129.13,58.5C125.47,55.19 124.87,55 118.13,55H111Z
      M119.88 136.77C107.13,151.54 107,151.68 107,150.29C107,149.67 108.57,145.42 110.5,140.86C112.43,136.3 114,132.21 114,131.78C114,131.35 111.26,131 107.92,131H101.83L105.14 126.82C106.97,124.52 110.07,121.03 112.04,119.07C114,117.11 117.56,113.12 119.93,110.21C122.31,107.3 124.44,105.11 124.68,105.35C124.91,105.58 123.51,109.73 121.55,114.58C119.6,119.42 118,123.53 118,123.69C118,123.86 120.7,124 124,124C127.3,124 130,124.24 130,124.52C130,124.81 125.45,130.33 119.88,136.77Z"
      fill="#092f60"
    />
  </svg>
);

const ZapIcon = () => (
  <svg viewBox="0 0 64 80" fill="none">
    {/* === Beine (unten, gespreizt) === */}
    <line x1="8" y1="78" x2="22" y2="48" stroke="#374151" strokeWidth="3.5" strokeLinecap="round"/>
    <line x1="56" y1="78" x2="42" y2="48" stroke="#374151" strokeWidth="3.5" strokeLinecap="round"/>

    {/* === Körper mitte === */}
    <line x1="22" y1="48" x2="26" y2="30" stroke="#374151" strokeWidth="3.5"/>
    <line x1="42" y1="48" x2="38" y2="30" stroke="#374151" strokeWidth="3.5"/>

    {/* === Körper oben === */}
    <line x1="26" y1="30" x2="29" y2="16" stroke="#374151" strokeWidth="3.5"/>
    <line x1="38" y1="30" x2="35" y2="16" stroke="#374151" strokeWidth="3.5"/>

    {/* === Dreieck-Spitze === */}
    <line x1="29" y1="16" x2="32" y2="5" stroke="#374151" strokeWidth="3.5" strokeLinecap="round"/>
    <line x1="35" y1="16" x2="32" y2="5" stroke="#374151" strokeWidth="3.5" strokeLinecap="round"/>
    {/* Kleine Kreuzstrebe im Dreieck */}
    <line x1="30" y1="12" x2="34" y2="16" stroke="#374151" strokeWidth="2"/>
    <line x1="34" y1="12" x2="30" y2="16" stroke="#374151" strokeWidth="2"/>

    {/* === Oberer Querträger === */}
    <line x1="4" y1="18" x2="60" y2="18" stroke="#374151" strokeWidth="3.5" strokeLinecap="round"/>
    {/* Schräge Stützen zum Querträger */}
    <line x1="28" y1="14" x2="10" y2="18" stroke="#374151" strokeWidth="2.5"/>
    <line x1="36" y1="14" x2="54" y2="18" stroke="#374151" strokeWidth="2.5"/>

    {/* Isolatoren oben (kleine Striche) */}
    <line x1="4" y1="19" x2="4" y2="23" stroke="#374151" strokeWidth="2.5" strokeLinecap="round"/>
    <line x1="9" y1="19" x2="9" y2="22" stroke="#374151" strokeWidth="2" strokeLinecap="round"/>
    <line x1="55" y1="19" x2="55" y2="22" stroke="#374151" strokeWidth="2" strokeLinecap="round"/>
    <line x1="60" y1="19" x2="60" y2="23" stroke="#374151" strokeWidth="2.5" strokeLinecap="round"/>

    {/* === X-Streben oben (zwischen Querträger und Mitte) === */}
    <line x1="26" y1="30" x2="37" y2="20" stroke="#374151" strokeWidth="2"/>
    <line x1="38" y1="30" x2="27" y2="20" stroke="#374151" strokeWidth="2"/>

    {/* === Unterer Querträger === */}
    <line x1="12" y1="36" x2="52" y2="36" stroke="#374151" strokeWidth="3.5" strokeLinecap="round"/>
    {/* Schräge Stützen zum unteren Querträger */}
    <line x1="25" y1="32" x2="16" y2="36" stroke="#374151" strokeWidth="2.5"/>
    <line x1="39" y1="32" x2="48" y2="36" stroke="#374151" strokeWidth="2.5"/>

    {/* Isolatoren unten (kleine Striche) */}
    <line x1="12" y1="37" x2="12" y2="41" stroke="#374151" strokeWidth="2.5" strokeLinecap="round"/>
    <line x1="17" y1="37" x2="17" y2="40" stroke="#374151" strokeWidth="2" strokeLinecap="round"/>
    <line x1="47" y1="37" x2="47" y2="40" stroke="#374151" strokeWidth="2" strokeLinecap="round"/>
    <line x1="52" y1="37" x2="52" y2="41" stroke="#374151" strokeWidth="2.5" strokeLinecap="round"/>

    {/* === X-Streben unten (zwischen Querträger und Beine) === */}
    <line x1="22" y1="48" x2="40" y2="38" stroke="#374151" strokeWidth="2"/>
    <line x1="42" y1="48" x2="24" y2="38" stroke="#374151" strokeWidth="2"/>
  </svg>
);

const EnergieFlussVisualisierung = () => {
  const [energieFluss, setEnergieFluss] = useState({
    pvProduktion: 0,
    hausVerbrauch: 0,
    batterieStand: 0,
    batterieLaden: 0,
    batterieEntladen: 0,
    autoLaden: 0,
    netzEinspeisung: 0,
    netzBezug: 0
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  // API-Endpoint - Passe diese URL an deine API an
  const API_URL = '/api/inverter/summary';

  // Daten von der API laden
  useEffect(() => {
    const fetchEnergieData = async () => {
      try {
        const response = await fetch(API_URL);
        
        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`);
        }
        
        const data = await response.json();
        
        // Hier die Datenstruktur an deine API anpassen
        setEnergieFluss({
          pvProduktion: data.ppv || 0,
          hausVerbrauch: data.house_consumption || 0,
          batterieStand: data.battery_soc || 0,
          batterieLaden: Math.max(0, -(parseFloat(data.pbattery) || 0)),
          batterieEntladen: Math.max(0, parseFloat(data.pbattery) || 0),
          autoLaden: parseFloat(data.car_power) || 0,
          netzEinspeisung: Math.max(0, (data.ppv || 0) - (data.house_consumption || 0) + (data.pbattery || 0)),
          netzBezug: Math.max(0, (data.house_consumption || 0) - (data.ppv || 0) - (data.pbattery || 0))
        });
        
        setLoading(false);
        setError(null);
      } catch (err) {
        console.error('Fehler beim Laden der Energiedaten:', err);
        setError(err.message);
        setLoading(false);
      }
    };

    // Initiales Laden
    fetchEnergieData();

    // Aktualisiere die Daten alle 5 Sekunden
    const interval = setInterval(fetchEnergieData, 5000);

    return () => clearInterval(interval);
  }, []);

  const FlussLinie = ({ von, nach, aktiv, leistung, farbe = "green" }) => {
    // Start, Kontrollpunkt (für Kurve), Ende
    const linien = {
      'pv-batterie':   { x1: 250, y1: 100, cx: 200, cy: 50, x2: 150, y2: 100 },
      'pv-haus':       { x1: 300, y1: 150, cx: 300, cy: 220, x2: 300, y2: 280 },
      'haus-auto':     { x1: 250, y1: 300, cx: 200, cy: 260, x2: 150, y2: 300 },
      'batterie-haus': { x1: 150, y1: 100, cx: 200, cy: 200, x2: 300, y2: 280 },
      'netz-haus':     { x1: 480, y1: 300, cx: 400, cy: 260, x2: 340, y2: 300 },
      'haus-netz':     { x1: 340, y1: 300, cx: 400, cy: 340, x2: 480, y2: 300 }
    };

    const linie = linien[`${von}-${nach}`];
    if (!linie || !aktiv) return null;

    const kurvenPfad = `M ${linie.x1},${linie.y1} Q ${linie.cx},${linie.cy} ${linie.x2},${linie.y2}`;
    const strokeColor = farbe === "green" ? "#10b981" : "#ef4444";

    return (
      <g>
        {/* Geschwungene Linie */}
        <path
          d={kurvenPfad}
          stroke={strokeColor}
          strokeWidth="3"
          fill="none"
          opacity="0.3"
        />
        {/* Animierter Punkt entlang der Kurve */}
        <circle r="6" fill={strokeColor}>
          <animateMotion
            dur="2s"
            repeatCount="indefinite"
            path={kurvenPfad}
          />
        </circle>
        {/* Leistungsanzeige in der Mitte der Kurve */}
        <text
          x={linie.cx}
          y={linie.cy - 0}
          fill="#64748b"
          fontSize="12"
          fontWeight="600"
          textAnchor="middle"
        >
          {leistung > 0 ? `${(leistung / 1000).toFixed(1)} kW` : ''}
        </text>
      </g>
    );
  };

  if (loading) {
    return (
      <div style={{
        minHeight: '100vh',
        background: 'linear-gradient(to bottom right, #f8fafc, #e2e8f0)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center'
      }}>
        <div style={{ textAlign: 'center' }}>
          <div style={{
            width: '50px',
            height: '50px',
            border: '5px solid #e2e8f0',
            borderTop: '5px solid #3b82f6',
            borderRadius: '50%',
            animation: 'spin 1s linear infinite',
            margin: '0 auto 1rem'
          }}></div>
          <p style={{ color: '#64748b' }}>Lade Energiedaten...</p>
        </div>
        <style>{`
          @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
          }
        `}</style>
      </div>
    );
  }

  if (error) {
    return (
      <div style={{
        minHeight: '100vh',
        background: 'linear-gradient(to bottom right, #f8fafc, #e2e8f0)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '2rem'
      }}>
        <div style={{
          background: 'white',
          borderRadius: '0.5rem',
          padding: '2rem',
          boxShadow: '0 4px 6px rgba(0,0,0,0.1)',
          maxWidth: '500px',
          textAlign: 'center'
        }}>
          <h2 style={{ color: '#ef4444', marginBottom: '1rem', fontSize: '1.5rem' }}>
            Fehler beim Laden der Daten
          </h2>
          <p style={{ color: '#64748b', marginBottom: '1rem' }}>{error}</p>
          <p style={{ color: '#64748b', fontSize: '0.875rem' }}>
            Bitte prüfe die API-URL in der Komponente : <code>{API_URL}</code>
          </p>
        </div>
      </div>
    );
  }

  return (
    <div style={{
      minHeight: '100vh',
      background: 'linear-gradient(to bottom right, #f8fafc, #e2e8f0)',
      padding: '2rem'
    }}>
      <div style={{ maxWidth: '1280px', margin: '0 auto' }}>
        <h1 style={{
          fontSize: '2.25rem',
          fontWeight: 'bold',
          color: '#1e293b',
          marginBottom: '0.5rem',
          textAlign: 'center'
        }}>
          Live Energiefluss 
        </h1>
        <p style={{
          color: '#64748b',
          textAlign: 'center',
          marginBottom: '2rem'
        }}>
          Live-Darstellung der Energieströme
        </p>

        <svg width="600" height="600" style={{ display: 'block', margin: '0 auto' }}>
        {/* PV-Anlage */}
        <g transform="translate(250, 50)">
          <circle cx="50" cy="50" r="50" fill="none" stroke="#000" strokeWidth="2" strokeOpacity="0.8">
            <animate attributeName="stroke-opacity" values="0.6;0.8;0.6" dur="2s" repeatCount="indefinite"/>
          </circle>
          <g transform="translate(5, 5) scale(0.15)" style={{ color: '#fff' }}>
            <SolarPanelIcon />
          </g>
          <text x="50" y="-10" textAnchor="middle" fill="#334155" fontWeight="600">
            PV-Anlage
          </text>
          <text x="50" y="138" textAnchor="middle" fill="#10b981" fontWeight="bold" fontSize="18">
            {(energieFluss.pvProduktion / 1000).toFixed(1)} kW
          </text>
        </g>

          {/* Batterie */}
          <g transform="translate(50, 50)">
          <circle cx="50" cy="50" r="50" fill="none" stroke="#000" strokeWidth="2" strokeOpacity="0.8">
            <animate attributeName="stroke-opacity" values="0.6;0.8;0.6" dur="2s" repeatCount="indefinite"/>
          </circle>
            <g transform="translate(5, 5)scale(0.15)" style={{ color: '#181515' }}>
              <BatteryIcon level={energieFluss.batterieStand} />
            </g>
            <text x="50" y="115" textAnchor="middle" fill="#334155" fontWeight="600">
              Batterie
            </text>
            <text x="50" y="135" textAnchor="middle" fill="#3b82f6" fontWeight="bold" fontSize="18">
              {energieFluss.batterieStand.toFixed()}%
            </text>
          </g>

          {/* Haus */}
          <g transform="translate(250, 250)">
          <circle cx="50" cy="50" r="50" fill="none" stroke="#000" strokeWidth="2" strokeOpacity="0.8">
            <animate attributeName="stroke-opacity" values="0.6;0.8;0.6" dur="2s" repeatCount="indefinite"/>
          </circle>
            <g transform="translate(5, 0)scale(0.15)" style={{ color: '#fff' }}>
              <HomeIcon />
            </g>
            <text x="50" y="115" textAnchor="middle" fill="#334155" fontWeight="600">
              Haus
            </text>
            <text x="50" y="135" textAnchor="middle" fill="#8b5cf6" fontWeight="bold" fontSize="18">
              {(energieFluss.hausVerbrauch / 1000).toFixed(1)} kW
            </text>
          </g>

          {/* Auto */}
          <g transform="translate(50, 250)">
            <circle cx="50" cy="50" r="50" fill="none" stroke="#000" strokeWidth="2" strokeOpacity="0.8">
              <animate attributeName="stroke-opacity" values="0.6;0.8;0.6" dur="2s" repeatCount="indefinite"/>
            </circle>
            <g transform="translate(5, 5) scale(0.15)">
              <CarIcon />
            </g>
            <text x="50" y="115" textAnchor="middle" fill="#334155" fontWeight="600">
              Enyaq Coupé
            </text>
            <text x="50" y="135" textAnchor="middle" fill="#1a1a1a" fontWeight="bold" fontSize="18">
              {energieFluss.autoLaden > 0 ? `${(energieFluss.autoLaden / 1000).toFixed(1)} kW` : 'Standby'}
            </text>
          </g>

          {/* Netz */}
          <g transform="translate(450, 250)">
            <circle cx="50" cy="50" r="50" fill="none" stroke="#000" strokeWidth="2" strokeOpacity="0.8">
              <animate attributeName="stroke-opacity" values="0.6;0.8;0.6" dur="2s" repeatCount="indefinite"/>
            </circle>
            <g transform="translate(20, 20) scale(0.1)" style={{ color: '#c9cc05' }}>
              <ZapIcon />
            </g>
            <text x="50" y="120" textAnchor="middle" fill="#334155" fontWeight="600">
              Netz
            </text>
            <text x="50" y="138" textAnchor="middle" fill={energieFluss.netzEinspeisung > 0 ? '#10b981' : '#ef4444'} fontWeight="bold" fontSize="18">
              {energieFluss.netzEinspeisung > 0 
                ? `↑ ${(energieFluss.netzEinspeisung / 1000).toFixed(1)} kW`
                : energieFluss.netzBezug > 0 
                  ? `↓ ${(energieFluss.netzBezug / 1000).toFixed(1)} kW`
                  : '0 kW'}
            </text>
          </g>

          {/* Energiefluss-Linien */}
          <FlussLinie 
            von="pv" 
            nach="batterie" 
            aktiv={energieFluss.batterieLaden > 0} 
            leistung={energieFluss.batterieLaden}
          />
          <FlussLinie 
            von="pv" 
            nach="haus" 
            aktiv={energieFluss.pvProduktion > 0} 
            leistung={Math.min(energieFluss.pvProduktion, energieFluss.hausVerbrauch)}
          />
          <FlussLinie 
            von="haus" 
            nach="auto" 
            aktiv={energieFluss.autoLaden > 0} 
            leistung={energieFluss.autoLaden}
            farbe="red"
          />
          <FlussLinie 
            von="batterie" 
            nach="haus" 
            aktiv={energieFluss.batterieEntladen > 0} 
            leistung={energieFluss.batterieEntladen}
            farbe="red"
          />
          <FlussLinie 
            von="haus" 
            nach="netz" 
            aktiv={energieFluss.netzEinspeisung > 0} 
            leistung={energieFluss.netzEinspeisung}
          />
          <FlussLinie 
            von="netz" 
            nach="haus" 
            aktiv={energieFluss.netzBezug > 0} 
            leistung={energieFluss.netzBezug}
            farbe="red"
          />
        </svg>

        <div style={{
          marginTop: '2rem',
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))',
          gap: '1rem'
        }}>
          <div style={{ background: 'white', borderRadius: '0.5rem', padding: '1rem', boxShadow: '0 1px 3px rgba(0,0,0,0.1)' }}>
            <div style={{ color: '#64748b', fontSize: '0.875rem', marginBottom: '0.25rem' }}>PV-Produktion</div>
            <div style={{ fontSize: '1.5rem', fontWeight: 'bold', color: '#10b981' }}>
              {(energieFluss.pvProduktion / 1000).toFixed(1)} kW
            </div>
          </div>
          <div style={{ background: 'white', borderRadius: '0.5rem', padding: '1rem', boxShadow: '0 1px 3px rgba(0,0,0,0.1)' }}>
            <div style={{ color: '#64748b', fontSize: '0.875rem', marginBottom: '0.25rem' }}>Hausverbrauch</div>
            <div style={{ fontSize: '1.5rem', fontWeight: 'bold', color: '#8b5cf6' }}>
              {(energieFluss.hausVerbrauch / 1000).toFixed(1)} kW
            </div>
          </div>
          <div style={{ background: 'white', borderRadius: '0.5rem', padding: '1rem', boxShadow: '0 1px 3px rgba(0,0,0,0.1)' }}>
            <div style={{ color: '#64748b', fontSize: '0.875rem', marginBottom: '0.25rem' }}>Batteriestand</div>
            <div style={{ fontSize: '1.5rem', fontWeight: 'bold', color: '#3b82f6' }}>
              {energieFluss.batterieStand.toFixed(0)}%
            </div>
          </div>
          <div style={{ background: 'white', borderRadius: '0.5rem', padding: '1rem', boxShadow: '0 1px 3px rgba(0,0,0,0.1)' }}>
            <div style={{ color: '#64748b', fontSize: '0.875rem', marginBottom: '0.25rem' }}>E-Auto</div>
            <div style={{ fontSize: '1.5rem', fontWeight: 'bold', color: '#ec4899' }}>
              {(energieFluss.autoLaden / 1000).toFixed(1)} kW
            </div>
          </div>
          <div style={{ background: 'white', borderRadius: '0.5rem', padding: '1rem', boxShadow: '0 1px 3px rgba(0,0,0,0.1)' }}>
            <div style={{ color: '#64748b', fontSize: '0.875rem', marginBottom: '0.25rem' }}>Netzbilanz</div>
            <div style={{ fontSize: '1.5rem', fontWeight: 'bold', color: energieFluss.netzEinspeisung > 0 ? '#10b981' : '#ef4444' }}>
              {energieFluss.netzEinspeisung > 0 
                ? `+${(energieFluss.netzEinspeisung / 1000).toFixed(1)} kW`
                : energieFluss.netzBezug > 0 
                  ? `-${(energieFluss.netzBezug / 1000).toFixed(1)} kW`
                  : '0 kW'}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default EnergieFlussVisualisierung;