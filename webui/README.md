# ETA Web UI (React)

Ziel: Einfache Weboberfläche mit Seitenleiste, um verschiedene Dashboards anzuzeigen.

## Frontend-Komponenten im Überblick

### App-Struktur

- `src/main.tsx`
	- Router-Bootstrap und App-Start.
	- Mountet `RouterProvider` auf `#root`.

- `src/App.tsx`
	- Globales Layout (Sidebar + Content-Bereich).
	- Definiert Navigation und `Outlet`-Container.
	- Enthält das Scroll-Layout (nur Content-Bereich scrollt).

### Seiten (`src/pages`)

- `DashboardHome.tsx`
	- Startseite/Ubersicht.

- `EnergyFlow.tsx`
	- Energiefluss-Visualisierung (PV/Verbrauch/Batterie).

- `Huehnerklappe.tsx`
	- Steuerung fur die Huhnerklappe (manuell + Schedule).
	- UI-State Synchronisierung mit Backend (`/api/huehnerklappe/ui-state`).
	- Schedule-Konfiguration inklusive Zeitstempel, Aktionen und Verlauf.

- `Heating.tsx` (+ `Heating.css`)
	- ETA-Heizungsansicht.

- `Grafana.tsx`
	- Einbindung/Verlinkung von Grafana-Ansichten.

- `goE.tsx`
	- Seite fur goE-/Wallbox-nahe Daten oder Steuerung.

- `UpdatePage.tsx`
	- UI fur Update-Prozesse.

- `Weather.tsx`
	- Wetterstation: aktuelle Messwerte von Weather Underground und Stunden-/Sonnenzeiten-Forecast von Open-Meteo.
	- Endpunkte, Variablen, Einheiten und Nutzungsbedingungen: [Wetter-API-Integration](../WEATHER_API.md).

- `Inverter.tsx`, `Wallbox.tsx`
	- Weitere Seitenkomponenten fur Inverter-/Wallbox-Funktionen (je nach aktueller Router-Konfiguration eingebunden oder vorbereitet).

### Gemeinsame Komponenten

- `src/components/Charts.tsx`
	- Wiederverwendbare Chart-Darstellung fur Dashboard-Seiten.

### API-Anbindung im Frontend

- Primär über REST-Endpunkte, z. B. `/api/huehnerklappe/*`, `/api/wallbox/*`, `/api/inverter/*` und `/api/weather/*`.
- Die Wetterseite liest `/api/weather/status`, speichert Konfiguration über `/api/weather/settings` und stößt Aktualisierungen über `/api/weather/refresh` an. Der Go-Backenddienst ruft die aktuelle PWS-Messung bei Weather Underground und den Forecast bei Open-Meteo ab.
- Entwicklungsbetrieb typischerweise uber Vite-Dev-Server, Produktionsbetrieb uber Build + statisches Hosting.

## Entwicklung

```
cd webui
npm install
npm run dev
# Öffnen: http://localhost:5173
```

## TypeScript prüfen

Mit dem TypeScript-Compiler können Syntax, Typen, Imports und JSX/TSX-Strukturen geprüft werden, ohne JavaScript-Dateien zu erzeugen:

```
npx tsc --noEmit
```

Dabei gilt:

- `npx` verwendet das lokal im Projekt installierte Paket.
- `tsc` startet den TypeScript-Compiler.
- `--noEmit` verhindert die Ausgabe von kompilierten JavaScript-Dateien und Source Maps.

Wenn der Befehl ohne Ausgabe mit Exit-Code `0` endet, wurden keine relevanten TypeScript-Fehler gefunden. Der Check bestätigt jedoch nicht, dass die Anwendung im Browser fehlerfrei läuft, API-Aufrufe funktionieren oder CSS und Layout korrekt sind. Dafür zusätzlich den Produktions-Build ausführen und die Anwendung im Browser testen.

## Build

```
npm run build
npm run preview
```

## TODO
- Karten/Charts (z.B. Recharts)
- Auth / Deployment
