package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
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

	deadline := time.Now().Add(10 * time.Second)
	for !mqttManager.IsConnected() {
		if time.Now().After(deadline) {
			log.Fatal("❌ MQTT connection timeout")
		}
		time.Sleep(200 * time.Millisecond)
	}

	if err := restpkg.DeleteVarSetFromConfig(restConfigPath); err != nil {
		log.Printf("⚠️  varset delete skipped: %v", err)
	} else {
		log.Println("🗑️  varset deleted")
	}

	payload, err := restpkg.PublishVariableSetOnce(restConfigPath, mqttManager, topic)
	if err != nil {
		log.Fatalf("❌ REST publish error: %v", err)
	}

	payloadJSON, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		log.Fatalf("❌ payload marshal error: %v", err)
	}
	log.Printf("📦 REST payload:\n%s", payloadJSON)
	if err := os.WriteFile("varset_payload.json", payloadJSON, 0644); err != nil {
		log.Fatalf("❌ write varset_payload.json: %v", err)
	}
	log.Println("📄 Variable set payload written to varset_payload.json")

	log.Println("📡 REST variable set published once")

	treeJSON, err := json.MarshalIndent(restpkg.GetEtaTree(), "", "  ")
	if err != nil {
		log.Fatalf("❌ etaTree marshal error: %v", err)
	}
	if err := os.WriteFile("eta_tree.json", treeJSON, 0644); err != nil {
		log.Fatalf("❌ write eta_tree.json: %v", err)
	}
	log.Println("📄 ETA tree written to eta_tree.json")
}
