package rest

import (
	"context"
	"log"
	"strconv"
	"time"

	"webgui-api/mqtt"
)

const sunTimesMQTTPrefix = "nano/esp32/suntime"

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

// @brief Publishes the cached sunrise/sunset values on six day-specific topics.
// @details
// Each local day receives its own retained sunrise and sunset values under
// nano/esp32/suntime/dayN/{sunrise|sunset}. Retained values let a sleeping
// ESP32 receive the latest times after reconnecting. Dates remain available in
// the weather status response; day1/day2/day3 are ordered from today forward.
// Empty snapshots are ignored, and individual publish failures are logged
// while remaining topics are still attempted.
// @param[in] sunTimes Three-day local sunrise/sunset snapshot.
func (rs *RestService) publishSunTimes(sunTimes []SunTimes) {
	if len(sunTimes) == 0 || rs.mqttManager == nil {
		return
	}
	if len(sunTimes) > 3 {
		sunTimes = sunTimes[:3]
	}
	for index, day := range sunTimes {
		dayTopic := sunTimesMQTTPrefix + "/day" + strconv.Itoa(index+1)
		for _, item := range []struct {
			name  string
			value string
		}{{name: "sunrise", value: day.Sunrise}, {name: "sunset", value: day.Sunset}} {
			if err := rs.mqttManager.PublishRetainedText(dayTopic+"/"+item.name, item.value); err != nil {
				log.Printf("[weather] MQTT publish failed for %s: %v", dayTopic+"/"+item.name, err)
			}
		}
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
