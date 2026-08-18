# ETA Web UI (React)

Ziel: Einfache Weboberfläche mit Seitenleiste, um verschiedene Dashboards anzuzeigen.

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
- Datenquellen anbinden (InfluxDB / REST)
- Karten/Charts (z.B. Recharts)
- Auth / Deployment
