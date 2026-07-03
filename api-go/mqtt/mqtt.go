package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

// / @brief Verwaltet MQTT-Verbindung, Subscriptions und das Empfangen von Nachrichten.
// /
// / Der Manager stellt eine threadsichere Map aller empfangenen Topics bereit
// / und verbindet sich automatisch neu, falls die Verbindung verloren geht.
type Manager struct {
	Broker string
	Port   int
	Topics []string

	mutex    sync.RWMutex
	received map[string]any
	client   pahomqtt.Client
	running  bool
}

// / @brief Enthält die Topic-Listen aus dem Abschnitt "topics" der Konfigurationsdatei.
// /
// / Jedes Feld entspricht einem Gerät/Namespace und wird beim Laden der Config
// / automatisch befüllt.
type brokerTopics struct {
	GoE      []string `json:"goE"`
	Goodwe   []string `json:"goodwe"`
	NanoMqtt []string `json:"nano_mqtt"`
}

// / @brief Spiegelt die Struktur der JSON-Konfigurationsdatei wider.
// /
// / Wird beim Einlesen der Datei per json.Unmarshal befüllt.
type brokerConfig struct {
	BrokerIP string       `json:"broker_ip"`
	Port     int          `json:"port"`
	Topics   brokerTopics `json:"topics"`
}

// / @brief Erstellt einen neuen MQTT-Manager aus einer Konfigurationsdatei.
// /
// / Sucht die Konfigurationsdatei in folgender Reihenfolge:
// /  1. übergebener configPath
// /  2. Umgebungsvariable MQTT_CONFIG
// /  3. broker_config.json im Arbeitsverzeichnis oder neben der Binary
// /
// / @param configPath Pfad zur JSON-Konfigurationsdatei; leer = automatische Suche.
// / @return Zeiger auf den initialisierten Manager oder ein Fehler beim Parsen der Config.
func NewManager(configPath string) (*Manager, error) {

	// 1. Config-Pfad finden, falls nicht angegeben
	if configPath == "" {
		if env := os.Getenv("MQTT_CONFIG"); env != "" {
			configPath = env
		} else {
			// Look in working directory, then next to executable
			candidates := []string{
				"broker_config.json",
				filepath.Join(filepath.Dir(os.Args[0]), "broker_config.json"),
			}
			for _, c := range candidates {
				if _, err := os.Stat(c); err == nil {
					configPath = c
					break
				}
			}
		}
	}

	// 2. Config-Datei lesen
	cfg := brokerConfig{BrokerIP: "localhost", Port: 1883}
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			log.Printf("[MQTT] config not found at %s, using defaults", configPath)
		} else {
			if err := json.Unmarshal(data, &cfg); err != nil {
				return nil, fmt.Errorf("config parse error: %w", err)
			}
			log.Printf("[MQTT] loaded config from %s", configPath)
		}
	}

	// 3. Collect all topics
	var topics []string
	topics = append(topics, cfg.Topics.GoE...)
	topics = append(topics, cfg.Topics.Goodwe...)
	topics = append(topics, cfg.Topics.NanoMqtt...)

	return &Manager{
		Broker:   cfg.BrokerIP,
		Port:     cfg.Port,
		Topics:   topics,
		received: make(map[string]any),
	}, nil
}

// / @brief Gibt eine threadsichere Kopie aller bisher empfangenen MQTT-Nachrichten zurück.
// /
// / Der Schlüssel der Map ist der abgeleitete Topic-Kurzname (z. B. "battery_soc"),
// / der Wert ist entweder ein geparster JSON-Wert oder ein Rohstring.
// /
// / @return Kopie der empfangenen Nachrichten-Map.
func (mqtt_manager *Manager) Messages() map[string]any {
	mqtt_manager.mutex.RLock()
	defer mqtt_manager.mutex.RUnlock()
	mqtt_map := make(map[string]any, len(mqtt_manager.received))
	for key, value := range mqtt_manager.received {
		mqtt_map[key] = value
	}
	return mqtt_map
}

// / @brief Gibt an, ob der MQTT-Client aktuell mit dem Broker verbunden ist.
// /
// / @return true wenn verbunden, false wenn nicht initialisiert oder getrennt.
func (mqtt_manager *Manager) IsConnected() bool {
	if mqtt_manager == nil || mqtt_manager.client == nil {
		return false
	}
	return mqtt_manager.client.IsConnected()
}

// / @brief Veröffentlicht eine Nachricht auf einem MQTT-Topic.
// /
// / Der Wert wird automatisch als JSON serialisiert, bevor er gesendet wird.
// / Gibt einen Fehler zurück, wenn der Client nicht verbunden ist.
// /
// / @param topic  Das MQTT-Topic, auf dem veröffentlicht werden soll.
// / @param value  Der zu sendende Wert (wird JSON-kodiert).
// / @return Fehler oder nil bei Erfolg.
func (mqtt_manager *Manager) Publish(topic string, value any) error {
	if mqtt_manager.client == nil || !mqtt_manager.client.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}
	// change value to JSON, mqtt can not handel go objects, pack to send
	payload, _ := json.Marshal(value)
	token := mqtt_manager.client.Publish(topic, 0, false, string(payload))
	token.Wait()
	if token.Error() != nil {
		return token.Error()
	}
	log.Printf("📤 Published to %s", topic)
	return nil
}

// / @brief Verbindet sich mit dem MQTT-Broker, abonniert alle konfigurierten Topics
// /        und empfängt Nachrichten dauerhaft in einer Schleife.
// /
// / Bei Verbindungsverlust wird automatisch nach einem konfigurierbaren Intervall
// / ein Reconnect-Versuch unternommen. Sollte als Goroutine gestartet werden:
// /
// /   go manager.Run()
// /
// / Läuft so lange, bis Stop() aufgerufen wird.
func (mqtt_manager *Manager) Run() {
	mqtt_manager.running = true
	reconnectInterval := 5 * time.Second

	for mqtt_manager.running {
		broker := fmt.Sprintf("tcp://%s:%d", mqtt_manager.Broker, mqtt_manager.Port)
		opts := pahomqtt.NewClientOptions().
			AddBroker(broker).
			SetClientID(fmt.Sprintf("webgui-api-%d", time.Now().UnixMilli())).
			SetAutoReconnect(true).
			SetConnectionLostHandler(func(client pahomqtt.Client, err error) {
				log.Printf("⚠️  MQTT connection lost: %v", err)
			}).
			SetOnConnectHandler(func(client pahomqtt.Client) {
				log.Printf("✅ Connected to MQTT broker %s", broker)
				// Subscribe on (re)connect
				for _, topic := range mqtt_manager.Topics {
					token := client.Subscribe(topic, 0, mqtt_manager.handleMessage)
					token.Wait()
					if token.Error() != nil {
						log.Printf("  ❌ Subscribe failed %s: %v", topic, token.Error())
					} else {
						log.Printf("  📥 Subscribed to %s", topic)
					}
				}
			})
		log.Printf("🔌 Connecting to MQTT broker at %s…", broker)
		mqtt_manager.client = pahomqtt.NewClient(opts)
		token := mqtt_manager.client.Connect()
		token.Wait()
		if token.Error() != nil {
			log.Printf("❌ MQTT connect error: %v, retrying in %s", token.Error(), reconnectInterval)
			mqtt_manager.client = nil
			time.Sleep(reconnectInterval)
			continue
		}

		// Block until stopped
		for mqtt_manager.running && mqtt_manager.client.IsConnected() {
			time.Sleep(1 * time.Second)
		}

		if mqtt_manager.client != nil {
			mqtt_manager.client.Disconnect(250)
			mqtt_manager.client = nil
		}

		if mqtt_manager.running {
			log.Printf("⚠️  Reconnecting in %s…", reconnectInterval)
			time.Sleep(reconnectInterval)
		}
	}

	log.Println("🛑 MQTT manager stopped")
}

// / @brief Beendet die Run-Schleife und trennt die Verbindung zum Broker.
// /
// / Setzt das running-Flag auf false und ruft Disconnect auf dem Client auf,
// / falls dieser noch initialisiert ist.
func (mqtt_manager *Manager) Stop() {
	mqtt_manager.running = false
	if mqtt_manager.client != nil {
		mqtt_manager.client.Disconnect(250)
	}
}

// / @brief Callback-Funktion für eingehende MQTT-Nachrichten.
// /
// / Versucht den Payload als JSON zu parsen; bei Misserfolg wird der Rohstring gespeichert.
// / Der Map-Schlüssel wird aus dem Topic abgeleitet, indem die ersten beiden Segmente
// / (Namespace + Geräte-ID) entfernt und die verbleibenden Segmente mit "_" verbunden werden.
// /
// / Beispiele:
// /   "goodwe/9020KETT/battery/soc" → "battery_soc"
// /   "go-eCharger/254959/nrg"      → "nrg"
// /
// / @param client  Der MQTT-Client, der die Nachricht empfangen hat (nicht verwendet).
// / @param msg     Die empfangene MQTT-Nachricht mit Topic und Payload.
func (mqtt_manager *Manager) handleMessage(client pahomqtt.Client, msg pahomqtt.Message) {
	topic := msg.Topic()
	payload := msg.Payload()

	// Try to parse as JSON
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		// Use raw string if not valid JSON
		value = string(payload)
	}

	// Build key: "goodwe/254959/battery/soc" → "battery_soc"
	//            "go-eCharger/254959/nrg"    → "nrg"
	parts := strings.Split(topic, "/")
	var key string
	if len(parts) > 2 {
		key = strings.Join(parts[2:], "_")
	} else {
		key = parts[len(parts)-1]
	}

	mqtt_manager.mutex.Lock()
	mqtt_manager.received[key] = value
	mqtt_manager.mutex.Unlock()
}
