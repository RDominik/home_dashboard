package rest

import (
	"context"
	"log"
	"time"

	"webgui-api/mqtt"
)

// RestService manages the REST client polling service
type RestService struct {
	ctx         context.Context
	cancel      context.CancelFunc
	done        chan struct{}
	configPath  string
	mqttManager *mqtt.Manager
	topic       string
	interval    time.Duration
}

// NewRestService creates a new RestService instance
// configPath: path to the rest config JSON file
// topic: MQTT topic to publish to
// interval: polling interval (default 60s if <= 0)
func NewRestService(configPath string, topic string, interval time.Duration) *RestService {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	return &RestService{
		configPath: configPath,
		topic:      topic,
		interval:   interval,
		done:       make(chan struct{}),
	}
}

// Start begins the REST polling service as a goroutine
func (rs *RestService) Start(mqttManager *mqtt.Manager) {
	if mqttManager == nil {
		log.Println("⚠️ REST service not started: mqtt manager is nil")
		return
	}
	rs.mqttManager = mqttManager
	rs.ctx, rs.cancel = context.WithCancel(context.Background())
	log.Printf("📡 REST service starting (interval: %v, topic: %s)...", rs.interval, rs.topic)

	go rs.runLoop()
}

// runLoop is the main service loop that polls and publishes
func (rs *RestService) runLoop() {
	defer func() {
		close(rs.done)
		log.Println("📡 REST service stopped")
	}()

	// Call PublishVariableSetLoop with the service's context
	if err := PublishVariableSetLoop(rs.ctx, rs.configPath, rs.mqttManager, rs.topic, rs.interval); err != nil {
		log.Printf("❌ REST service error: %v", err)
	}
}

// Stop gracefully stops the REST service
func (rs *RestService) Stop() {
	if rs.cancel != nil {
		log.Println("📡 Stopping REST service...")
		rs.cancel()
		<-rs.done
	}
}
