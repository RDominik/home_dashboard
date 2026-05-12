package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"
	"time"

	"webgui-api/mqtt"
	restpkg "webgui-api/rest"
)

func main() {
	var restConfigPath string
	var mqttConfigPath string
	var topic string
	var interval time.Duration

	flag.StringVar(&restConfigPath, "rest-config", "rest/rest_config.json", "path to the REST config JSON")
	flag.StringVar(&mqttConfigPath, "mqtt-config", "mqtt/broker_config.json", "path to the MQTT config JSON")
	flag.StringVar(&topic, "topic", "webgui/rest/varset", "MQTT topic for the REST service")
	flag.DurationVar(&interval, "interval", 60*time.Second, "polling interval")
	flag.Parse()

	mqttManager, err := mqtt.NewManager(mqttConfigPath)
	if err != nil {
		log.Fatalf("❌ MQTT config error: %v", err)
	}
	go mqttManager.Run()
	defer mqttManager.Stop()

	service := restpkg.NewRestService(restConfigPath, topic, interval)
	service.Start(mqttManager)
	defer service.Stop()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("📡 REST service is running, waiting for shutdown signal...")
	<-ctx.Done()
	log.Println("📡 Shutdown signal received")
}
