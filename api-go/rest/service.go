package rest

import (
	"context"
	"log"
	"time"

	"webgui-api/mqtt"
)

const sunTimesMQTTTopic = "nano/esp32/sun-times"

// @brief Owns the REST polling lifecycle and its dependent services.
// @details
// RestService coordinates the ETA polling loop, MQTT publication, and the
// WeatherService lifecycle. It intentionally keeps transport-independent state
// and domain behavior in their respective modules while providing one lifecycle
// owner for application startup and shutdown.
type RestService struct {
	ctx         context.Context
	cancel      context.CancelFunc
	done        chan struct{}
	configPath  string
	mqttManager *mqtt.Manager
	topic       string
	interval    time.Duration
	weather     *WeatherService
}

// @brief Creates a RestService and all REST-backed services.
// @details
// The constructor normalizes an invalid polling interval to sixty seconds,
// creates the persistent WeatherService, and prepares the shutdown channel.
// MQTT is intentionally attached later by Start so construction remains free of
// broker side effects and callers can handle initialization errors explicitly.
// @param[in] configPath Path to the REST client configuration JSON file.
// @param[in] topic MQTT topic used for published ETA values.
// @param[in] interval ETA polling interval; values less than or equal to zero use 60 seconds.
// @return Initialized RestService, or an error when the WeatherService cannot be created.
func NewRestService(configPath string, topic string, interval time.Duration) (*RestService, error) {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	weather, err := NewWeatherService(weatherDBPath)
	if err != nil {
		return nil, err
	}
	return &RestService{
		configPath: configPath,
		topic:      topic,
		interval:   interval,
		done:       make(chan struct{}),
		weather:    weather,
	}, nil
}

// @brief Starts REST and Weather polling in background goroutines.
// @details
// A nil MQTT manager is rejected before any goroutine is started. For a valid
// manager, the method stores the dependency, creates the cancellation context,
// starts WeatherService polling, and launches the ETA publication loop.
// @param[in] mqttManager MQTT manager required for ETA publication.
func (rs *RestService) Start(mqttManager *mqtt.Manager) {
	if mqttManager == nil {
		log.Println("⚠️ REST service not started: mqtt manager is nil")
		return
	}
	rs.mqttManager = mqttManager
	rs.ctx, rs.cancel = context.WithCancel(context.Background())
	rs.weather.SetSunTimesPublisher(rs.publishSunTimes)
	// Republish the persisted snapshot on startup so the ESP32 receives current
	// values even when the next daily forecast refresh is still rate-limited.
	rs.publishSunTimes(rs.weather.GetSunTimes())
	rs.weather.Start()
	log.Printf("📡 REST service starting (interval: %v, topic: %s)...", rs.interval, rs.topic)

	go rs.runLoop()
}

// @brief Publishes the cached three-day sunrise/sunset payload to the ESP32.
// @details
// The payload is a JSON array of local date, sunrise, and sunset values on the
// nano/esp32 MQTT namespace. Empty snapshots are ignored; publication errors
// are logged without interrupting weather polling.
// @param[in] sunTimes Three-day local sunrise/sunset snapshot.
func (rs *RestService) publishSunTimes(sunTimes []SunTimes) {
	if len(sunTimes) == 0 || rs.mqttManager == nil {
		return
	}
	if err := rs.mqttManager.PublishRetained(sunTimesMQTTTopic, sunTimes); err != nil {
		log.Printf("[weather] MQTT sun-times publish failed: %v", err)
	}
}

// @brief Runs the ETA polling and MQTT publication loop.
// @details
// The loop delegates polling, transformation, and publication to
// PublishVariableSetLoop. It always closes the completion channel on exit so
// Stop can wait for graceful termination and logs any loop-level error.
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

// @brief Gracefully stops REST and Weather polling and closes persistence.
// @details
// When Start has launched the service, cancellation is requested first and the
// method waits for the ETA loop to finish. Weather polling is then stopped so
// its database can be closed only after its background goroutine has exited.
func (rs *RestService) Stop() {
	if rs.cancel != nil {
		log.Println("📡 Stopping REST service...")
		rs.cancel()
		<-rs.done
	}
	if rs.weather != nil {
		rs.weather.Stop()
	}
}
