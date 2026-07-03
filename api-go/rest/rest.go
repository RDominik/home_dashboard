package rest

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"webgui-api/mqtt"
)

type Eta_menu struct {
	XMLName xml.Name `xml:"eta"`
	Version string   `xml:"version,attr"`
	XMLNS   string   `xml:"xmlns,attr"`
	Menu    Menu     `xml:"menu"`
}

type Menu struct {
	Fub Fub `xml:"fub"`
}

type Fub struct {
	URI     string   `xml:"uri,attr"`
	Name    string   `xml:"name,attr"`
	Objects []Object `xml:"object"` // Erste Ebene der Objekte
}

type Object struct {
	URI     string   `xml:"uri,attr"`
	Name    string   `xml:"name,attr"`
	Objects []Object `xml:"object"` // Rekursive Einbettung für Sub-Objekte
	Value   Value    `xml:"value"`
}

type Read struct {
	XMLName xml.Name `xml:"eta"`
	Version string   `xml:"version,attr"`
	XMLNS   string   `xml:"xmlns,attr"`
	Value   Value    `xml:"value"`
}

type Value struct {
	XMLName       xml.Name `xml:"value"`
	URI           string   `xml:"uri,attr"`
	StrValue      string   `xml:"strValue,attr"`
	Unit          string   `xml:"unit,attr"`
	DecPlaces     string   `xml:"decPlaces,attr"`
	ScaleFactor   string   `xml:"scaleFactor,attr"`
	AdvTextOffset string   `xml:"advTextOffset,attr"`
	Text          string   `xml:",chardata"`
}

type Varset_Head struct {
	XMLName  xml.Name `xml:"eta"`
	Version  string   `xml:"version,attr"`
	XMLNS    string   `xml:"xmlns,attr"`
	Variable Varset   `xml:"vars"`
}

type Varset struct {
	XMLName xml.Name          `xml:"vars"`
	URI     string            `xml:"uri,attr"`
	Objects []Varset_Variable `xml:"variable"` // Variablen im Varset-Response
}

type Varset_Variable struct {
	XMLName       xml.Name `xml:"variable"`
	URI           string   `xml:"uri,attr"`
	Name          string   `xml:"-" json:"Name,omitempty"`
	StrValue      string   `xml:"strValue,attr"`
	Unit          string   `xml:"unit,attr"`
	DecPlaces     string   `xml:"decPlaces,attr"`
	ScaleFactor   string   `xml:"scaleFactor,attr"`
	AdvTextOffset string   `xml:"advTextOffset,attr"`
	Text          string   `xml:",chardata"`
}

// Varset_Put represents the response from the API when creating a variable set.
type Varset_Put struct {
	XMLName xml.Name `xml:"eta"`
	Version string   `xml:"version,attr"`
	XMLNS   string   `xml:"xmlns,attr"`
	Error   ErrorMsg `xml:"error"`
}

type ErrorMsg struct {
	XMLName xml.Name `xml:"error"`
	URI     string   `xml:"uri,attr"`
	Text    string   `xml:",chardata"`
}

type RestClient struct {
	Client string `json:"client_ip"`
	Port   int    `json:"port"`
	Varset string `json:"varset"`
}

var httpClient = &http.Client{}

// NewRest creates a REST client from the JSON configuration.
// @param configPath Path to the JSON configuration file. If empty, REST_CONFIG,
// then rest_config.json in the current working directory, and then next to the
// executable are checked.
// @return A configured RestClient instance or an error if the configuration is invalid.
func NewRest(configPath string) (*RestClient, error) {

	// Resolve config path from explicit argument, environment, or known defaults.
	if configPath == "" {
		if env := os.Getenv("REST_CONFIG"); env != "" {
			configPath = env
		} else {
			// Look in working directory, then next to executable.
			candidates := []string{
				"rest_config.json",
				filepath.Join(filepath.Dir(os.Args[0]), "rest_config.json"),
			}
			for _, c := range candidates {
				if _, err := os.Stat(c); err == nil {
					configPath = c
					break
				}
			}
		}
	}

	// Load config file when available, otherwise keep safe defaults.
	cfg := RestClient{Client: "localhost", Port: 8080, Varset: "myVar1"}
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			log.Printf("[REST] config not found at %s, using defaults", configPath)
		} else {
			if err := json.Unmarshal(data, &cfg); err != nil {
				return nil, fmt.Errorf("config parse error: %w", err)
			}
			log.Printf("[REST] loaded config from %s", configPath)
		}
	}

	// Validate configuration early to avoid invalid URLs at runtime.
	if cfg.Client == "" {
		return nil, fmt.Errorf("client_ip must not be empty")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535, got %d", cfg.Port)
	}
	if cfg.Varset == "" {
		return nil, fmt.Errorf("varset must not be empty")
	}

	return &RestClient{
		Client: cfg.Client,
		Port:   cfg.Port,
		Varset: cfg.Varset,
	}, nil
}

// baseURL builds the base REST endpoint from host and port.
// @return Base URL in the form http://<host>:<port>.
func (r *RestClient) baseURL() string {
	return fmt.Sprintf("http://%s:%d", r.Client, r.Port)
}

// MenuURL builds the menu endpoint URL for the ETA device.
// @return URL to /user/menu.
func (r *RestClient) MenuURL() string {
	return r.baseURL() + "/user/menu"
}

// menuURL is the unexported alias kept for internal callers.
func (r *RestClient) menuURL() string {
	return r.MenuURL()
}

// varsetURL builds the configured variable set endpoint URL.
// @return URL to /user/vars/<varset>.
func (r *RestClient) varsetURL() string {
	return fmt.Sprintf("%s/user/vars/%s", r.baseURL(), r.Varset)
}

// BuildVarSetFromConfig loads the REST target from JSON configuration and creates the configured variable set.
// @param configPath Path to the JSON configuration file.
// @return An error if loading the configuration, reading the menu, or creating the variable set fails.
func BuildVarSetFromConfig(configPath string) error {
	// Load target settings from config first.
	client, err := NewRest(configPath)
	if err != nil {
		return err
	}

	// Parse menu and create/fill the configured variable set.
	var eta Eta_menu
	return buildVarSetFromETAMenu(client.menuURL(), client.varsetURL(), &eta)
}

// ParseETAMenuToVarSet reads an ETA menu from the given URL and populates a variable set.
// @param url Source URL for the ETA menu XML.
// @param menu Destination object that receives the parsed menu.
// @return An error if the menu cannot be fetched or parsed, or if the variable set cannot be created.
func ParseETAMenuToVarSet(url string, menu *Eta_menu) error {
	// Backward-compatible default for legacy callers.
	variableSet := "http://192.168.188.99:8080/user/vars/myset1"
	return buildVarSetFromETAMenu(url, variableSet, menu)
}

// BuildURINameMap recursively walks the ETA menu tree and returns a map of
// URI (without leading "/") to the display name of each menu object.
func BuildURINameMap(menu *Eta_menu) map[string]string {
	result := make(map[string]string)
	for _, obj := range menu.Menu.Fub.Objects {
		collectURINames(obj, result)
	}
	return result
}

// collectURINames is the recursive helper for BuildURINameMap.
func collectURINames(obj Object, m map[string]string) {
	uri := strings.TrimPrefix(obj.URI, "/")
	if uri != "" && obj.Name != "" {
		m[uri] = obj.Name
	}
	for _, child := range obj.Objects {
		collectURINames(child, m)
	}
}

// FetchETAMenu fetches and parses the ETA menu XML from the given URL.
// @param url Source URL for the ETA menu XML (e.g. http://<host>:<port>/user/menu).
// @return The parsed Eta_menu structure or an error if fetching or parsing fails.
func FetchETAMenu(url string) (Eta_menu, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Eta_menu{}, err
	}
	req.Header.Set("Accept", "application/xml, application/json, text/xml, */*")

	resp, err := httpClient.Do(req)
	if err != nil {
		return Eta_menu{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Eta_menu{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return Eta_menu{}, fmt.Errorf("status %d: %s", resp.StatusCode, snippet)
	}

	var menu Eta_menu
	if err := xml.Unmarshal(body, &menu); err != nil {
		return Eta_menu{}, fmt.Errorf("parse error: %w", err)
	}
	return menu, nil
}

// buildVarSetFromETAMenu fetches the ETA menu XML and creates/populates a variable set.
// @param url Source URL for menu XML.
// @param variableSet Target variable set URL.
// @param menu Destination structure for parsed menu data.
// @return An error if fetching, parsing, or creating the variable set fails.
func buildVarSetFromETAMenu(url string, variableSet string, menu *Eta_menu) error {
	// Fetch and parse menu via shared helper.
	fetched, err := FetchETAMenu(url)
	if err != nil {
		return err
	}
	*menu = fetched

	// Ensure the target variable set exists.
	warning, err := createVarSetEntry(variableSet, "")
	if err != nil {
		return fmt.Errorf("error when creating variable set: %w", err)
	}
	if warning != "" {
		// Existing set is acceptable for this flow.
		log.Printf("warning when creating variable set: %s", warning)
		return nil
	}

	// Add all discovered menu objects recursively to the set.
	log.Printf("variable set created successfully")
	addObjectsToVarSet(variableSet, menu.Menu.Fub.Objects)

	return nil
}

// createVarSetEntry issues a PUT to create a variable set or add a single variable path.
// @param url Base variable set URL.
// @param value Optional object URI suffix to add to the set.
// @return Warning text for non-fatal API messages and an error for hard failures.
func createVarSetEntry(url string, value string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Compose endpoint: base varset URL plus optional object URI.
	myValue := url + value
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, myValue, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/xml, application/json, text/xml, */*")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Parse API response to detect known warning conditions.
	var out Varset_Put
	err = xml.Unmarshal([]byte(body), &out)
	if err != nil {
		log.Printf("error parsing response: %v", err)
		return "", err
	}
	if out.Error.Text == "Variable set already exists" {
		return out.Error.Text, nil
	}

	log.Printf("variable set created: %s", string(body))
	return "", nil
}

// addObjectsToVarSet recursively adds all object URIs from menu nodes to a variable set.
// @param varSet Target variable set URL.
// @param in Input object tree.
func addObjectsToVarSet(varSet string, in []Object) {

	for _, obj := range in {
		// Add current node first, then recurse into children.
		createVarSetEntry(varSet, obj.URI)
		if obj.Objects != nil {
			addObjectsToVarSet(varSet, obj.Objects)
		}
	}
}

// deriveMenuURLFromVarSetURL converts a varset URL to the corresponding /user/menu URL.
// @param varSetURL URL to /user/vars/<name>.
// @return Derived menu URL or an error if the input does not match the expected format.
func deriveMenuURLFromVarSetURL(varSetURL string) (string, error) {
	idx := strings.Index(varSetURL, "/user/vars/")
	if idx == -1 {
		return "", fmt.Errorf("cannot derive menu URL from varset URL: %s", varSetURL)
	}

	return varSetURL[:idx] + "/user/menu", nil
}

// RequestVarSet reads a variable set from the given URL into the provided output structure.
// @param url Source URL of the variable set.
// @param out Destination structure for the parsed XML response.
// @return An error if the request fails or the HTTP response is not successful.
func RequestVarSet(url string, out *Varset_Head) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// Always release timeout context resources in this function.
	defer cancel()

	// Always perform a GET (the API returns XML)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	// indicate we accept XML (and JSON as fallback)
	req.Header.Set("Accept", "application/xml, application/json, text/xml, */*")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}

	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return readErr
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusNotFound {
			// Auto-create once and let caller read on next call.
			menuURL, err := deriveMenuURLFromVarSetURL(url)
			if err != nil {
				return err
			}

			var eta Eta_menu
			if err := buildVarSetFromETAMenu(menuURL, url, &eta); err != nil {
				return fmt.Errorf("auto-create variable set failed: %w", err)
			}

			log.Printf("variable set created, will be available on next RequestVarSet call")
			return nil
		}

		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return fmt.Errorf("status %d: %s", resp.StatusCode, snippet)
	}

	if out != nil {
		// Parse successful XML payload into caller structure.
		err := xml.Unmarshal(body, out)
		if err != nil {
			return fmt.Errorf("error parsing variable set response: %w", err)
		}
		log.Printf("variable set response parsed successfully")
	}

	return nil
}

// VarPayload is the slim per-variable payload published to MQTT.
type VarPayload struct {
	Name          string `json:"Name"`
	StrValue      string `json:"StrValue"`
	Unit          string `json:"Unit"`
	DecPlaces     string `json:"DecPlaces"`
	ScaleFactor   string `json:"ScaleFactor"`
	AdvTextOffset string `json:"AdvTextOffset"`
}

// PublishVariableSetOnce requests the configured variable set once and publishes
// each variable individually to its own MQTT topic (eta/<URI>).
// @param restConfigPath Path to REST JSON configuration.
// @param mqttManager MQTT manager instance used for publish calls.
// @param topic Unused – kept for interface compatibility; topic is derived as "eta/<URI>".
// @return The fetched (and name-enriched) variable set payload and an error if
//
//	configuration is invalid, mqttManager is nil, reading fails, or any publish fails.
func PublishVariableSetOnce(restConfigPath string, mqttManager *mqtt.Manager, topic string) (Varset_Head, error) {
	// Load REST endpoint configuration (includes predefined variable set name).
	restClient, err := NewRest(restConfigPath)
	if err != nil {
		return Varset_Head{}, err
	}

	if mqttManager == nil {
		return Varset_Head{}, fmt.Errorf("mqtt manager must not be nil")
	}

	var payload Varset_Head
	if err := RequestVarSet(restClient.varsetURL(), &payload); err != nil {
		return Varset_Head{}, err
	}

	// Enrich variables with display names from the ETA menu.
	menu, err := FetchETAMenu(restClient.menuURL())
	if err != nil {
		log.Printf("[REST] could not fetch menu for name mapping: %v", err)
	} else {
		uriNameMap := BuildURINameMap(&menu)
		for i := range payload.Variable.Objects {
			if name, ok := uriNameMap[payload.Variable.Objects[i].URI]; ok {
				payload.Variable.Objects[i].Name = name
			}
		}
	}

	// Publish each variable value to its own topic: "eta/<URI>/<field>"
	for _, v := range payload.Variable.Objects {
		baseTopic := "eta/" + v.URI

		// Publish each field as a separate topic with scalar value
		fields := map[string]string{
			"Name":          v.Name,
			"StrValue":      v.StrValue,
			"Unit":          v.Unit,
			"DecPlaces":     v.DecPlaces,
			"ScaleFactor":   v.ScaleFactor,
			"AdvTextOffset": v.AdvTextOffset,
		}

		for fieldName, fieldValue := range fields {
			topic := baseTopic + "/" + fieldName
			if err := mqttManager.Publish(topic, fieldValue); err != nil {
				log.Printf("[REST] publish failed for topic %s: %v", topic, err)
			}
		}
	}

	log.Printf("published %d variables to MQTT (prefix eta/)", len(payload.Variable.Objects))
	return payload, nil
}

// PublishVariableSetLoop requests the configured variable set in a periodic loop
// and publishes the response to the provided MQTT topic.
// @param ctx Cancellation context controlling the lifetime of the polling loop.
// @param restConfigPath Path to REST JSON configuration.
// @param mqttManager MQTT manager instance used for publish calls.
// @param topic MQTT topic used to publish the variable set payload.
// @param interval Polling interval. If <= 0, 60s is used.
// @return An error if configuration is invalid or mqttManager is nil.
func PublishVariableSetLoop(ctx context.Context, restConfigPath string, mqttManager *mqtt.Manager, topic string, interval time.Duration) error {
	// Use default polling period when caller does not provide one.
	if interval <= 0 {
		interval = 60 * time.Second
	}

	// pollAndPublish executes one full cycle: read varset and publish it to MQTT.
	pollAndPublish := func() {
		if _, err := PublishVariableSetOnce(restConfigPath, mqttManager, topic); err != nil {
			log.Printf("request variable set failed: %v", err)
			return
		}
	}

	// Run once immediately so callers do not wait for the first interval tick.
	pollAndPublish()

	// Ticker emits a signal on ticker.C every configured interval.
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Keep running until the caller cancels the context.
	for {
		select {
		// Context cancellation is the stop signal for this long-running loop.
		case <-ctx.Done():
			return nil
		// Each ticker event triggers the next read-and-publish cycle.
		case <-ticker.C:
			pollAndPublish()
		}
	}
}

// FetchWallboxVarSet performs a sample GET request against the configured wallbox endpoint.
func FetchWallboxVarSet() {
	var eta Eta_menu
	// Example call path kept for manual verification/debugging.
	log.Printf("fetching API request...")
	if err := FetchURLResponse("http://192.168.188.99:8080/user/vars/myset1", nil, &eta); err != nil {
		log.Printf("error on API request: %v", err)
		return
	}
	log.Printf("API request completed")
}

// FetchURLResponse performs a GET request and optionally stores the raw or parsed response in dest.
// @param url Source URL to request.
// @param payload Unused placeholder for compatibility with older call sites.
// @param dest Optional destination for the response body.
// @return An error if the request fails or the HTTP response is not successful.
func FetchURLResponse(url string, payload any, dest any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Perform a GET request and optionally copy raw response into dest.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	// indicate we accept XML (and JSON as fallback)
	req.Header.Set("Accept", "application/xml, application/json, text/xml, */*")

	resp, err := httpClient.Do(req)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return fmt.Errorf("status %d: %s", resp.StatusCode, snippet)
	}

	if dest != nil {
		// Support convenient raw targets: *string and *[]byte.
		rv := reflect.ValueOf(dest)
		if rv.Kind() == reflect.Ptr && !rv.IsNil() {
			elem := rv.Elem()
			if elem.Kind() == reflect.String {
				elem.SetString(string(body))
				return nil
			}
			if elem.Kind() == reflect.Slice && elem.Type().Elem().Kind() == reflect.Uint8 {
				elem.SetBytes(body)
				return nil
			}
		}
	}
	return nil
}
