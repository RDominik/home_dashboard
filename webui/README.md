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
	- Klappenansicht mit gleichrangigen Tabs für manuelle Steuerung, Sleep-Schedule und Testmodus.
	- UI-State Synchronisierung mit Backend (`/api/huehnerklappe/ui-state`).
	- Schedule-Konfiguration inklusive Zeitstempel, Aktionen und Verlauf.
	- Persistierter Testmodus mit einstellbarem Intervall (1–1440 Minuten), separater Maximalwachzeit (1–86400 Sekunden) und täglichem Zeitfenster in der Schedule-Zeitzone; die erste Aktion ist Öffnen nach Ablauf des ersten Intervalls, danach wechseln Öffnen und Schließen. Nach Ablauf der Wachzeit schläft der Controller nur bis zur nächsten Intervallgrenze, damit die Wachzeit das Aktionsintervall nicht zusätzlich verlängert.
	- Ein eigener Testmodus-Verlauf zeigt die letzten 20 Zyklen einschließlich Aktion, Startzeit, Position, Motorlaufzeit, Akku-Rohwert, Wachzeit, Sleep-Befehl sowie Schlaf- und Aufwachbestätigung. Einstellungen werden über die UI-State-API serverseitig gespeichert; Verlauf und Runtime-Zustand bleiben in bbolt erhalten.
	- Das Zeitfenster umfasst den Start, aber nicht das Ende und kann Mitternacht überschreiten. Bei aktivem Timestamp-Schedule pausiert der Testmodus; ein bei nicht verfügbarem Controller fälliger Einzelauftrag wird bis zur Wiederverbindung gehalten.

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
