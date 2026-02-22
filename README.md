# eta

## Docker Start (Web UI + API)

Schnellstart lokal mit Docker Compose:

```
docker compose -f docker-compose.webui.yml up --build -d
# Web UI:   http://localhost:8080
# API (opt): http://localhost:8081
```
logs der docker container
docker compose -f docker-compose.webui.yml logs -f api

Nur Web UI bauen/starten (ohne Compose):

```
# Image bauen
docker build -f webui/Dockerfile -t eta-webui .
# Starten
docker run --rm -p 8080:8080 eta-webui
```

Nur API bauen/starten (ohne Compose):

```
# Image bauen
docker build -f api/Dockerfile -t eta-api .
# Starten
docker run --rm -p 8081:8081 eta-api
# Test: http://localhost:8081/api/inverter/summary
```

## MQTT-Anbindung (API → WebUI)

Die API kann Live‑Werte (PV/Haushalt/SoC) per MQTT abonnieren und als REST an die WebUI liefern.

Env‑Variablen (docker-compose.webui.yml, überschreibbar):

```
MQTT_HOST=test.mosquitto.org
MQTT_PORT=1883
# optional
MQTT_USER=
MQTT_PASS=

# Topics → Felder
MQTT_TOPIC_PPV=eta/ppv              # → ppv (W)
MQTT_TOPIC_HOUSE=eta/house          # → house_consumption (W)
MQTT_TOPIC_SOC=eta/battery/soc      # → battery_soc (%)
```

Beispiel: Test mit öffentlichem Broker (ohne TLS)

```
# Publish Beispielwerte (in neuem Terminal)
mosquitto_pub -h test.mosquitto.org -t eta/ppv -m 2300
mosquitto_pub -h test.mosquitto.org -t eta/house -m 1200
mosquitto_pub -h test.mosquitto.org -t eta/battery/soc -m 57

# API‑Werte prüfen
curl -fsS http://localhost:8081/api/inverter/summary | jq
```

Hinweise
- Fallback: Ohne MQTT oder ohne Nachrichten liefert die API Start‑Mockwerte.
- Für produktive Nutzung bitte eigenen Broker + Auth/TLS konfigurieren und Topics anpassen.

