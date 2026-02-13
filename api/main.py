import os
import asyncio
from fastapi import FastAPI, Query
from fastapi.middleware.cors import CORSMiddleware
from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional

try:
    from asyncio_mqtt import Client as MqttClient
except Exception:  # library optional at runtime
    MqttClient = None  # type: ignore

app = FastAPI(title="ETA API", version="0.1.0")

# CORS: allow all for simplicity; restrict in production
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

def now_iso():
    return datetime.utcnow().isoformat() + 'Z'

# --- MQTT wiring (optional) ---
class InverterState:
    def __init__(self):
        self.ppv: float = 2400.0
        self.house_consumption: float = 1200.0
        self.battery_soc: float = 56.0
        self.ts: str = now_iso()
        self.lock = asyncio.Lock()

    async def update(self, **vals):
        async with self.lock:
            for k, v in vals.items():
                setattr(self, k, v)
            self.ts = now_iso()

    async def snapshot(self) -> Dict[str, Any]:
        async with self.lock:
            return {
                "timestamp": self.ts,
                "ppv": self.ppv,
                "house_consumption": self.house_consumption,
                "battery_soc": self.battery_soc,
            }

state = InverterState()
_mqtt_task: Optional[asyncio.Task] = None

async def _mqtt_runner():
    host = os.getenv("MQTT_HOST", "test.mosquitto.org")
    port = int(os.getenv("MQTT_PORT", "1883"))
    user = os.getenv("MQTT_USER") or None
    password = os.getenv("MQTT_PASS") or None
    topic_ppv = os.getenv("MQTT_TOPIC_PPV", "eta/ppv")
    topic_house = os.getenv("MQTT_TOPIC_HOUSE", "eta/house")
    topic_soc = os.getenv("MQTT_TOPIC_SOC", "eta/battery/soc")
    if not MqttClient:
        return  # library not installed, keep mocks
    while True:
        try:
            auth = {"username": user, "password": password} if user and password else {}
            async with MqttClient(hostname=host, port=port, **auth) as client:
                await client.subscribe([(topic_ppv, 0), (topic_house, 0), (topic_soc, 0)])
                async with client.unfiltered_messages() as messages:
                    async for msg in messages:
                        payload = msg.payload.decode(errors="ignore").strip()
                        try:
                            val = float(payload)
                        except Exception:
                            # basic JSON/path parsing could be added later
                            continue
                        if msg.topic == topic_ppv:
                            await state.update(ppv=val)
                        elif msg.topic == topic_house:
                            await state.update(house_consumption=val)
                        elif msg.topic == topic_soc:
                            await state.update(battery_soc=val)
        except Exception:
            # backoff and retry
            await asyncio.sleep(3)

@app.on_event("startup")
async def _startup():
    global _mqtt_task
    _mqtt_task = asyncio.create_task(_mqtt_runner())

@app.on_event("shutdown")
async def _shutdown():
    global _mqtt_task
    if _mqtt_task:
        _mqtt_task.cancel()
        try:
            await _mqtt_task
        except Exception:
            pass

@app.get("/api/wallbox/status")
async def wallbox_status() -> Dict[str, Any]:
    return {
        "timestamp": now_iso(),
        "amp": 8,
        "frc": 0,
        "psm": 0,
        "car": 2,
        "nrg": [0]*11 + [1234],
        "modelStatus": 1,
    }

@app.get("/api/wallbox/history")
async def wallbox_history(
    from_: Optional[str] = Query(None, alias="from"),
    to: Optional[str] = None,
    interval: str = "5m",
):
    points: List[Dict[str, Any]] = []
    end = datetime.utcnow()
    start = end - timedelta(hours=2)
    t = start
    while t <= end:
        points.append({
            "t": t.isoformat() + 'Z',
            "amp": 6 + ((t.minute % 5) - 2),
            "currentEnergy": 1000 + (t.minute * 3),
        })
        t += timedelta(minutes=5)
    return {"series": points, "interval": interval}

@app.get("/api/inverter/summary")
async def inverter_summary():
    return await state.snapshot()

@app.get("/api/inverter/history")
async def inverter_history(
    from_: Optional[str] = Query(None, alias="from"),
    to: Optional[str] = None,
    interval: str = "5m",
):
    points: List[Dict[str, Any]] = []
    end = datetime.utcnow()
    start = end - timedelta(hours=6)
    t = start
    while t <= end:
        minute = t.minute
        points.append({
            "t": t.isoformat() + 'Z',
            "ppv": 1500 + (minute * 5) % 1200,
            "house": 800 + (minute * 3) % 600,
            "battery_soc": 40 + (minute % 30) * 0.2,
        })
        t += timedelta(minutes=5)
    return {"series": points, "interval": interval}

# --- ETA Hackschnitzel-Heizung (Mock) ---
@app.get("/api/heating/summary")
async def heating_summary() -> Dict[str, Any]:
    return {
        "timestamp": now_iso(),
        "boiler_temp": 72.5,      # Kesseltemperatur (°C)
        "buffer_top": 68.3,       # Puffer oben (°C)
        "buffer_bottom": 45.8,    # Puffer unten (°C)
        "return_temp": 52.1,      # Rücklauf (°C)
        "outside_temp": 3.4,      # Außentemperatur (°C)
        "feed_rate": 35,          # Hackschnitzel-Förderrate (%)
        "burner_status": "on",   # on/off
    }

@app.get("/api/heating/history")
async def heating_history(
    from_: Optional[str] = Query(None, alias="from"),
    to: Optional[str] = None,
    interval: str = "5m",
):
    points: List[Dict[str, Any]] = []
    end = datetime.utcnow()
    start = end - timedelta(hours=12)
    t = start
    while t <= end:
        minute = t.minute
        points.append({
            "t": t.isoformat() + 'Z',
            "boiler_temp": 70 + (minute % 10) * 0.4,
            "buffer_top": 66 + (minute % 8) * 0.3,
            "buffer_bottom": 44 + (minute % 6) * 0.25,
            "return_temp": 50 + (minute % 12) * 0.2,
            "feed_rate": 30 + (minute % 5) * 3,
        })
        t += timedelta(minutes=5)
    return {"series": points, "interval": interval}
