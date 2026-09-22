import type { CSSProperties, ReactNode } from 'react'

type PageHeaderProps = {
  title: string
  subtitle: string
  eyebrow: string
  status?: string
  statusDetail?: string
  statusMessage?: ReactNode
  actions?: ReactNode
  bottomContent?: ReactNode
}

export default function PageHeader({
  title,
  subtitle,
  eyebrow,
  status = 'SYSTEM ONLINE',
  statusDetail = 'Aktuelle Ansicht und Datenquelle bereit',
  statusMessage,
  actions,
  bottomContent,
}: PageHeaderProps) {
  return (
    <header style={headerStyle}>
      <div style={headerRowStyle}>
        <div>
          <div style={eyebrowStyle}>{eyebrow}</div>
          <h1 style={titleStyle}>{title}</h1>
          <p style={subtitleStyle}>{subtitle}</p>
        </div>
        {actions}
      </div>
      <div style={statusBarStyle}>
        <span style={statusStyle}>{status}</span>
        <span style={statusDetailStyle}>{statusDetail}</span>
      </div>
      {statusMessage && <div style={statusMessageStyle}>{statusMessage}</div>}
      {bottomContent}
    </header>
  )
}

export function pageHeaderButtonStyle(background: string, color: string): CSSProperties {
  return {
    border: '1px solid #b7d7d2',
    borderRadius: 3,
    background,
    color,
    padding: '9px 13px',
    fontWeight: 800,
    cursor: 'pointer',
  }
}

const headerStyle: CSSProperties = {
  margin: '-20px -20px 20px',
  padding: '18px 24px 0',
  background: '#263d52',
  color: '#fff',
  boxShadow: '0 2px 8px rgba(21, 39, 53, 0.18)',
}

const headerRowStyle: CSSProperties = {
  display: 'flex',
  justifyContent: 'space-between',
  gap: 16,
  alignItems: 'center',
  flexWrap: 'wrap',
}

const eyebrowStyle: CSSProperties = {
  color: '#d8e3eb',
  fontFamily: 'Arial, sans-serif',
  fontSize: 12,
  fontWeight: 700,
  letterSpacing: 1.4,
}

const titleStyle: CSSProperties = {
  margin: '5px 0 3px',
  fontFamily: 'Georgia, "Times New Roman", serif',
  fontSize: 30,
  fontWeight: 500,
}

const subtitleStyle: CSSProperties = {
  margin: 0,
  color: '#bfccd6',
  fontFamily: 'Arial, sans-serif',
  fontSize: 13,
}

const statusBarStyle: CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 18,
  flexWrap: 'wrap',
  marginTop: 18,
  padding: '11px 0 12px',
  borderTop: '1px solid rgba(216, 227, 235, 0.22)',
  fontFamily: 'Arial, sans-serif',
  fontSize: 13,
}

const statusStyle: CSSProperties = {
  color: '#fff',
  fontWeight: 700,
}

const statusDetailStyle: CSSProperties = {
  color: '#c5d2dc',
}

const statusMessageStyle: CSSProperties = {
  padding: '0 0 12px',
  color: '#ffcfca',
  fontFamily: 'Arial, sans-serif',
  fontSize: 12,
}
