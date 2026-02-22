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

// Manager handles MQTT subscriptions and publishing.
type Manager struct {
	Broker string
	Port   int
	Topics []string

	mutex    sync.RWMutex
	received map[string]any
	client   pahomqtt.Client
	running  bool
}

// brokerTopics holds the topic lists nested under "topics" in the JSON.
type brokerTopics struct {
	GoE    []string `json:"goE"`
	Goodwe []string `json:"goodwe"`
}

// brokerConfig mirrors the JSON config file structure.
type brokerConfig struct {
	BrokerIP string       `json:"broker_ip"`
	Port     int          `json:"port"`
	Topics   brokerTopics `json:"topics"`
}

// NewManager creates a Manager from a config file.
// If configPath is empty, it looks for MQTT_CONFIG env or broker_config.json next to the binary.
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

	return &Manager{
		Broker:   cfg.BrokerIP,
		Port:     cfg.Port,
		Topics:   topics,
		received: make(map[string]any),
	}, nil
}

// Messages returns a copy of all received messages.
func (mqtt_manager *Manager) Messages() map[string]any {
	mqtt_manager.mutex.RLock()
	defer mqtt_manager.mutex.RUnlock()
	mqtt_map := make(map[string]any, len(mqtt_manager.received))
	for key, value := range mqtt_manager.received {
		mqtt_map[key] = value
	}
	return mqtt_map
}

// IsConnected reports whether the MQTT client is currently connected.
func (mqtt_manager *Manager) IsConnected() bool {
	if mqtt_manager == nil || mqtt_manager.client == nil {
		return false
	}
	return mqtt_manager.client.IsConnected()
}

// Publish sends a message to a topic.
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

// Run connects to the broker, subscribes, and listens forever.
// Call this in a goroutine: go manager.Run()
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

// Stop signals the run loop to exit.
func (mqtt_manager *Manager) Stop() {
	mqtt_manager.running = false
	if mqtt_manager.client != nil {
		mqtt_manager.client.Disconnect(250)
	}
}

// handleMessage processes incoming MQTT messages.
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
