import asyncio
import json
from pathlib import Path
from typing import Any, Dict, Optional, Union
import os

import aiomqtt


class MQTTManager:
    """Async MQTT Service using aiomqtt.

    - Subscribes to topics from JSON config
    - Stores latest message values per short key (last segment of topic)
    - Provides getters, publish, and an async run loop
    """

    def __init__(self, config_path: Union[str, Path, None] = None):
        # determine config path: env override -> provided -> module dir/broker_config.json
        env_path = os.environ.get("MQTT_CONFIG")
        if config_path:
            cfg_path = Path(config_path)
        elif env_path:
            cfg_path = Path(env_path)
        else:
            cfg_path = Path(__file__).parent / "broker_config.json"

        try:
            config = self._load_config(cfg_path)
        except FileNotFoundError:
            print(f"[MQTTManager] config not found at {cfg_path!s}, continuing with defaults")
            config = {}

        # broker IP (fallbacks)
        self.broker: str = config.get("broker_ip") or config.get("broker") or "localhost"
        self.port: int = int(config.get("port", 1883))

        # collect all list-valued items from the JSON
        self.topics: list[str] = []
        for section, entries in config.items():
            if section in ("broker_ip", "broker", "port"):
                continue
            if isinstance(entries, list):
                for item in entries:
                    if isinstance(item, str):
                        self.topics.append(item)

        self._received: Dict[str, Any] = {}
        self._lock = asyncio.Lock()
        self._client: Optional[aiomqtt.Client] = None
        self._running = False

    # ------------------------------------------------------------------
    # Config loader
    # ------------------------------------------------------------------
    @staticmethod
    def _load_config(path: Union[str, Path]) -> dict:
        p = Path(path)
        if not p.exists():
            raise FileNotFoundError(f"Config file not found: {p}")
        with p.open("r", encoding="utf-8") as f:
            return json.load(f)

    # ------------------------------------------------------------------
    # Public getters / setters
    # ------------------------------------------------------------------
    @property
    def message(self) -> Dict[str, Any]:
        """Return a shallow copy of the received messages dict (non-blocking)."""
        return dict(self._received)

    async def message_async(self) -> Dict[str, Any]:
        """Return received messages with lock protection."""
        async with self._lock:
            return dict(self._received)

    async def publish(self, topic: str, payload: Any, qos: int = 0, retain: bool = False):
        """Publish a message. The client must be connected (run() active)."""
        if self._client is None:
            raise RuntimeError("MQTT client is not connected – call run() first")
        await self._client.publish(topic, payload=str(payload), qos=qos, retain=retain)
        print(f"📤 Published to {topic}")

    async def set_keys(self, data: list, qos: int = 0, retain: bool = False):
        """Publish a list of (topic, value) pairs."""
        for topic, value in data:
            await self.publish(topic, value, qos=qos, retain=retain)

    # ------------------------------------------------------------------
    # Main async loop
    # ------------------------------------------------------------------
    async def run(self):
        """Connect to the broker, subscribe, and listen for messages forever.

        Call this as an asyncio task:
            asyncio.create_task(mqtt_manager.run())
        """
        self._running = True
        reconnect_interval = 5  # seconds

        while self._running:
            try:
                async with aiomqtt.Client(self.broker, self.port) as client:
                    self._client = client
                    print(f"✅ Connected to MQTT broker {self.broker}:{self.port}")

                    for topic in self.topics:
                        await client.subscribe(topic)
                        print(f"  📥 Subscribed to {topic}")

                    async for msg in client.messages:
                        await self._handle_message(msg)

            except aiomqtt.MqttError as err:
                self._client = None
                print(f"⚠️  MQTT connection lost ({err}), reconnecting in {reconnect_interval}s …")
                await asyncio.sleep(reconnect_interval)
            except asyncio.CancelledError:
                break
            except Exception as exc:
                self._client = None
                print(f"❌ Unexpected MQTT error: {exc!r}, reconnecting in {reconnect_interval}s …")
                await asyncio.sleep(reconnect_interval)

        self._client = None
        print("🛑 MQTT manager stopped")

    # ------------------------------------------------------------------
    # Stop
    # ------------------------------------------------------------------
    async def stop(self):
        """Signal the run-loop to exit."""
        self._running = False

    # ------------------------------------------------------------------
    # Internal
    # ------------------------------------------------------------------
    async def _handle_message(self, msg: aiomqtt.Message):
        topic = str(msg.topic)
        try:
            payload = msg.payload.decode()
            payload = json.loads(payload)
        except (UnicodeDecodeError, json.JSONDecodeError, AttributeError):
            payload = msg.payload

        # Build a key from everything after "device/id/..." prefix
        # e.g. "goodwe/254959/battery/soc"  → "battery_soc"
        #      "go-eCharger/254959/nrg"     → "nrg"
        parts = topic.split("/")
        key = "_".join(parts[2:]) if len(parts) > 2 else parts[-1]

        async with self._lock:
            self._received[key] = payload
