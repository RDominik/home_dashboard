import os
import asyncio
from fastapi import FastAPI, Query
from fastapi.middleware.cors import CORSMiddleware
from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional
from datetime import datetime, timezone
from mqtt_client import MQTTManager
from contextlib import asynccontextmanager

mqtt_client = MQTTManager()

# Hintergrund-Task-Referenz
_mqtt_task: Optional[asyncio.Task] = None
car_charging = 0


@asynccontextmanager
async def lifespan(app: FastAPI):
    global _mqtt_task
    # starte MQTT-Client als async Task
    _mqtt_task = asyncio.create_task(mqtt_client.run())
    try:
        yield
    finally:
        # stoppe MQTT sauber
        await mqtt_client.stop()
        if _mqtt_task:
            _mqtt_task.cancel()
            try:
                await _mqtt_task
            except asyncio.CancelledError:
                pass

app = FastAPI(title="ETA API", version="0.1.0", lifespan=lifespan)

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


# go-eCharger MQTT SET topic prefix
GOE_SET_PREFIX = "go-eCharger/254959"
# Erlaubte Keys zum Setzen (Whitelist)
GOE_ALLOWED_KEYS = {"amp", "frc", "psm", "dwo", "alw"}



@app.get("/api/wallbox/status")
async def wallbox_status() -> Dict[str, Any]:
    global mqtt_client
    status = await mqtt_client.message_async()    
    return {
        "timestamp": now_iso(),
        "amp": status.get("amp", 0),
        "frc": status.get("frc", 0),
        "psm": status.get("psm", 0),
        "car": status.get("car", 0),
        "nrg": status.get("nrg", [0]*16),
        "modelStatus": status.get("modelStatus", 0),
    }


from pydantic import BaseModel

class WallboxSetRequest(BaseModel):
    key: str
    value: Any

@app.post("/api/wallbox/set")
async def wallbox_set(req: WallboxSetRequest) -> Dict[str, Any]:
    """SET einen Wert am go-eCharger über MQTT."""
    if req.key not in GOE_ALLOWED_KEYS:
        return {"ok": False, "error": f"Key '{req.key}' nicht erlaubt. Erlaubt: {GOE_ALLOWED_KEYS}"}
    
    topic = f"{GOE_SET_PREFIX}/{req.key}/set"
    try:
        await mqtt_client.publish(topic, req.value)
        return {"ok": True, "topic": topic, "key": req.key, "value": req.value}
    except RuntimeError as e:
        return {"ok": False, "error": str(e)}

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
    global mqtt_client
    status = await mqtt_client.message_async()
    print("🔍 Inverter summary requested, current message:", status)
    nrg = status.get("nrg") if isinstance(status.get("nrg"), list) else []
    car_power = nrg[11] if len(nrg) > 11 else 0
    return {
        "timestamp": now_iso(),
        "ppv": status.get("ppv", 0),
        "house_consumption": status.get("house_consumption", 0),
        "battery_soc": status.get("battery_soc", 0),
        "pbattery": status.get("pbattery", 0),
        "car_power": car_power,
    }


# --- System Update (git pull + rebuild) ---
import subprocess

@app.post("/api/system/update")
async def system_update():
    """Git pull + Docker Compose rebuild auf dem Host."""
    repo_dir = "/repo"
    results = []

    # 1. git pull
    try:
        git = subprocess.run(
            ["git", "pull"],
            cwd=repo_dir,
            capture_output=True, text=True, timeout=60
        )
        results.append({
            "step": "git pull",
            "ok": git.returncode == 0,
            "stdout": git.stdout.strip(),
            "stderr": git.stderr.strip(),
        })
        if git.returncode != 0:
            return {"ok": False, "results": results}
    except Exception as e:
        return {"ok": False, "results": [{"step": "git pull", "ok": False, "stderr": str(e)}]}

    # 2. docker compose build + up
    try:
        build = subprocess.run(
            ["docker", "compose", "-f", "docker-compose.webui.yml", "up", "--build", "-d"],
            cwd=repo_dir,
            capture_output=True, text=True, timeout=300
        )
        results.append({
            "step": "docker compose up --build -d",
            "ok": build.returncode == 0,
            "stdout": build.stdout.strip()[-500:] if build.stdout else "",
            "stderr": build.stderr.strip()[-500:] if build.stderr else "",
        })
    except Exception as e:
        results.append({"step": "docker compose up --build -d", "ok": False, "stderr": str(e)})

    all_ok = all(r["ok"] for r in results)
    return {"ok": all_ok, "results": results}


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
