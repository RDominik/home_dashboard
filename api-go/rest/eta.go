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
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"webgui-api/mqtt"
)

type EtaMenu struct {
	XMLName xml.Name       `xml:"eta"`
	Version string         `xml:"version,attr"`
	XMLNS   string         `xml:"xmlns,attr"`
	Menu    EtaMenuSection `xml:"menu"`
}

type EtaMenuSection struct {
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

type topicValueStore struct {
	mu     sync.RWMutex
	values map[string]string
}

// nestedStore builds a hierarchical map[string]any from slash-separated topic paths.
type nestedStore struct {
	mu   sync.RWMutex
	root map[string]any
}

var etaTree = &nestedStore{root: make(map[string]any)}

// publishedValues hält den zuletzt erfolgreich gepublishten Wert pro Topic.
// Damit werden bei PublishVariableSetOnce und publishMenuTopics nur Topics
// tatsächlich an den Broker gesendet, deren Wert sich seit dem letzten
// Publish-Zyklus geändert hat.
var publishedValues = newTopicValueStore(256)

// @brief Stores a scalar value in the hierarchical ETA in-memory tree.
// @details
// This method splits a slash-separated topic path and walks or creates nested
// map levels until the final segment is reached. Existing non-map values on
// intermediate levels are replaced with fresh child maps to keep the tree
// structurally valid for subsequent nested inserts.
//
// The operation is fully synchronized with an exclusive lock to guarantee
// thread-safe writes when concurrent publishers update values.
// @param[in] topic Slash-separated topic path that identifies the target node.
// @param[in] value Scalar value stored at the leaf identified by topic.
func (s *nestedStore) set(topic string, value string) {
	parts := strings.Split(topic, "/")
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.root
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return
		}
		next, ok := current[part]
		if !ok {
			child := make(map[string]any)
			current[part] = child
			current = child
			continue
		}
		child, ok := next.(map[string]any)
		if !ok {
			child = make(map[string]any)
			current[part] = child
		}
		current = child
	}
}

// @brief Returns a deep-copied snapshot of the complete nested ETA tree.
// @details
// The method acquires a read lock, clones all nested map levels recursively,
// and returns the detached copy. Callers can read or modify the returned map
// without affecting internal shared state or requiring additional locks.
// @return Independent deep copy of the internal root map.
func (s *nestedStore) snapshot() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return deepCopyMap(s.root)
}

// @brief Provides a thread-safe snapshot of the global ETA value tree.
// @details
// This helper is the public access point for reading the aggregated ETA topic
// state. Internally it delegates to the store snapshot routine, ensuring the
// caller always receives a detached deep copy.
// @return Hierarchical map containing the current ETA values.
func GetEtaTree() map[string]any {
	return etaTree.snapshot()
}

// @brief Recursively deep-copies a map[string]any structure.
// @details
// Each entry is copied by value. Nested map[string]any values are copied
// recursively, while non-map values are transferred as-is. This function is
// used to build immutable read snapshots from shared in-memory state.
// @param[in] m Source map that should be cloned.
// @return New map instance with recursively copied nested maps.
func deepCopyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if child, ok := v.(map[string]any); ok {
			out[k] = deepCopyMap(child)
		} else {
			out[k] = v
		}
	}
	return out
}

// @brief Creates a synchronized topic-value store with optional capacity hint.
// @details
// The store is used as a temporary staging area for MQTT topic/value pairs.
// Providing a realistic size hint can reduce map reallocations when many
// entries are inserted in short time.
// @param[in] sizeHint Expected number of entries to store.
// @return Initialized pointer to topicValueStore.
func newTopicValueStore(sizeHint int) *topicValueStore {
	return &topicValueStore{values: make(map[string]string, sizeHint)}
}

// @brief Inserts or updates one value in the topic-value store.
// @details
// The method acquires an exclusive lock and writes the provided key/value pair.
// Existing entries for the same key are overwritten atomically.
// @param[in] key Topic key that should be updated.
// @param[in] value Scalar value associated with key.
func (s *topicValueStore) set(key string, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
}

// @brief Reads one value from the topic-value store.
// @details
// A shared read lock is used for concurrent-safe access. The boolean return
// indicates whether the key existed at lookup time.
// @param[in] key Topic key to retrieve.
// @return Stored value and a flag that is true when key exists.
func (s *topicValueStore) get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.values[key]
	return value, ok
}

// @brief Creates a filtered snapshot of topic/value entries.
// @details
// The method returns a detached copy containing all entries whose keys match
// the given prefix. If prefix is empty, all current entries are copied.
// @param[in] prefix Optional key prefix filter.
// @return New map containing matching key/value entries.
func (s *topicValueStore) snapshot(prefix string) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string)
	for key, value := range s.values {
		if prefix == "" || strings.HasPrefix(key, prefix) {
			out[key] = value
		}
	}
	return out
}

// @brief HTTP endpoint returning the full ETA in-memory tree as JSON.
// @details
// This handler serves /api/heating/summary and serializes the complete nested
// ETA state that is continuously filled by the publish loop. It intentionally
// returns the entire tree so frontend clients can select and aggregate values
// according to their own view requirements.
// @param[in,out] w HTTP response writer used for JSON output.
// @param[in] r Incoming HTTP request (unused except for endpoint context).
func HeatingSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(etaTree.snapshot())
}

// @brief HTTP endpoint returning synthetic recent heating history points.
// @details
// This handler serves /api/heating/history and currently generates synthetic
// time-series values for the previous 12 hours in 5-minute steps. The optional
// query parameter interval is echoed back for client display/selection logic.
//
// The generated payload is intended as fallback/demo data until persistent
// history storage is connected.
// @param[in,out] w HTTP response writer used for JSON output.
// @param[in] r Incoming HTTP request carrying optional query parameters.
func HeatingHistory(w http.ResponseWriter, r *http.Request) {
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "5m"
	}

	var points []map[string]any
	end := time.Now().UTC()
	start := end.Add(-12 * time.Hour)
	t := start
	for !t.After(end) {
		min := t.Minute()
		points = append(points, map[string]any{
			"t":             t.Format("2006-01-02T15:04:05.000Z"),
			"boiler_temp":   70.0 + float64(min%10)*0.4,
			"buffer_top":    66.0 + float64(min%8)*0.3,
			"buffer_bottom": 44.0 + float64(min%6)*0.25,
			"return_temp":   50.0 + float64(min%12)*0.2,
			"feed_rate":     30 + (min%5)*3,
		})
		t = t.Add(5 * time.Minute)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"series": points, "interval": interval})
}

var httpClient = &http.Client{}

const etaMenuFileName = "eta_menu.json"

const (
	menuPublishPauseEvery = 25
	menuPublishPause      = 100 * time.Millisecond
	mqttReconnectTimeout  = 5 * time.Second
)

// @brief Creates and validates a RestClient from JSON configuration.
// @details
// The function resolves the configuration source in this order: explicit
// configPath argument, REST_CONFIG environment variable, then known defaults.
// If no readable config is found, safe defaults are used. Parsed values are
// validated to prevent malformed runtime URLs and missing required fields.
//
// Validation failures are returned as errors so callers can fail fast during
// startup.
// @param[in] configPath Path to the JSON configuration file. If empty, REST_CONFIG,
// then rest_config.json in the current working directory, and then next to the
// executable are checked.
// @param[out] RestClient Configured REST client object on success.
// @return Pointer to a configured RestClient, or an error if parsing/validation fails.
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

// @brief Builds the base REST endpoint URL from the RestClient object.
// @details
// This helper centralizes host/port formatting and is reused by endpoint
// builders to ensure consistent URL generation across all REST requests.
// @param[in] r RestClient object used as source for host and port.
// @return Base URL in the form http://<host>:<port>.
func (r *RestClient) baseURL() string {
	return fmt.Sprintf("http://%s:%d", r.Client, r.Port)
}

// @brief Builds the ETA menu endpoint URL from the RestClient object.
// @details
// The returned endpoint targets the ETA menu root and is used for reading the
// structural object tree, deriving URI mappings, and rebuilding variable sets.
// @param[in] r RestClient object that provides base URL context.
// @return URL to /user/menu.
func (r *RestClient) MenuURL() string {
	return r.baseURL() + "/user/menu"
}

// @brief Internal alias for MenuURL for package-local callers.
// @details
// This method keeps backward-compatible internal call sites readable while
// routing all menu URL generation through the same public helper.
// @param[in] r RestClient object that provides base URL context.
// @return URL to /user/menu.
func (r *RestClient) menuURL() string {
	return r.MenuURL()
}

// @brief Builds the configured variable set endpoint URL from the RestClient object.
// @details
// The URL combines base endpoint and configured varset name and is the central
// read/write endpoint for variable-set lifecycle and polling operations.
// @param[in] r RestClient object containing host, port, and varset name.
// @return URL to /user/vars/<varset>.
func (r *RestClient) varsetURL() string {
	return fmt.Sprintf("%s/user/vars/%s", r.baseURL(), r.Varset)
}

// @brief Loads REST configuration and creates the configured variable set on the ETA device.
// @details
// The function builds a validated REST client, fetches the current ETA menu,
// and ensures the configured variable set exists and is populated from menu
// objects. It is typically used for initialization/bootstrap workflows.
// @param[in] configPath Path to the JSON configuration file.
// @return Error if loading configuration, reading menu, or creating variable set fails.
func BuildVarSetFromConfig(configPath string) error {
	// Load target settings from config first.
	client, err := NewRest(configPath)
	if err != nil {
		return err
	}

	// Parse menu and create/fill the configured variable set.
	var eta EtaMenu
	return buildVarSetFromETAMenu(client.menuURL(), client.varsetURL(), &eta)
}

// @brief Reads an ETA menu and populates a variable set using a legacy default target.
// @details
// This compatibility helper keeps historical call paths functional by using a
// fixed default variable-set URL. New integrations should prefer configured
// paths via BuildVarSetFromConfig.
// @param[in] url Source URL for the ETA menu XML.
// @param[out] menu Destination object that receives the parsed menu tree.
// @return Error if menu fetch/parse or variable set creation fails.
func ParseETAMenuToVarSet(url string, menu *EtaMenu) error {
	// Backward-compatible default for legacy callers.
	variableSet := "http://192.168.188.99:8080/user/vars/myset1"
	return buildVarSetFromETAMenu(url, variableSet, menu)
}

// @brief Builds a lookup map from normalized URI to display name based on the menu tree.
// @details
// The generated map is used to enrich raw variable-set payloads with readable
// names for MQTT publication and UI rendering.
// @param[in] menu Parsed ETA menu tree.
// @return Map of normalized URI to menu object display name.
func BuildURINameMap(menu *EtaMenu) map[string]string {
	result := make(map[string]string)
	for _, obj := range menu.Menu.Fub.Objects {
		collectURINames(obj, result)
	}
	return result
}

// @brief Builds a mapping from normalized URI to hierarchical MQTT topic path.
// @details
// Topic paths are derived from object names along the menu hierarchy. The
// resulting map allows stable URI-based payloads to be published under
// human-readable MQTT topics.
// @param[in] menu Parsed ETA menu tree.
// @return Map where key is normalized URI and value is name-based topic path.
func BuildURITopicMap(menu *EtaMenu) map[string]string {
	result := make(map[string]string)
	for _, obj := range menu.Menu.Fub.Objects {
		collectURITopics(obj, "", result)
	}
	return result
}

// @brief Recursively fills the URI-to-name lookup map for menu objects.
// @details
// Each object contributes one normalized URI -> display-name entry when both
// fields are available. Child objects are traversed depth-first.
// @param[in] obj Current menu object node.
// @param[in,out] m Destination map for normalized URI to display name.
func collectURINames(obj Object, m map[string]string) {
	uri := normalizeTopicURI(obj.URI)
	if uri != "" && obj.Name != "" {
		m[uri] = obj.Name
	}
	for _, child := range obj.Objects {
		collectURINames(child, m)
	}
}

// @brief Recursively builds name-based hierarchical topic paths for each URI.
// @details
// The function composes a topic path from sanitized object names along the
// current recursion branch and stores that path under the normalized URI key.
// @param[in] obj Current menu object node.
// @param[in] parentTopic Topic path built from ancestor objects.
// @param[in,out] m Destination map for normalized URI to hierarchical topic path.
func collectURITopics(obj Object, parentTopic string, m map[string]string) {
	segment := sanitizeTopicSegment(obj.Name)
	topicPath := parentTopic
	if segment != "" {
		if topicPath == "" {
			topicPath = segment
		} else {
			topicPath += "/" + segment
		}
	}

	uri := normalizeTopicURI(obj.URI)
	if uri != "" && topicPath != "" {
		m[uri] = topicPath
	}

	for _, child := range obj.Objects {
		collectURITopics(child, topicPath, m)
	}
}

// @brief Normalizes an ETA URI for stable key lookup and topic usage.
// @details
// Leading slashes are removed and trailing /0 segments are stripped repeatedly
// to collapse semantically equivalent URI variants into one canonical key.
// @param[in] uri Raw URI from ETA menu or variable set payload.
// @return URI without leading slash and trailing /0 segments.
func normalizeTopicURI(uri string) string {
	normalized := strings.TrimPrefix(strings.TrimSpace(uri), "/")
	for strings.HasSuffix(normalized, "/0") {
		normalized = strings.TrimSuffix(normalized, "/0")
	}
	return normalized
}

// @brief Converts a menu display name to a safe MQTT topic segment.
// @details
// The routine trims surrounding whitespace, replaces slashes to avoid topic
// hierarchy conflicts, and normalizes internal whitespace to underscores.
// @param[in] name Raw menu object name.
// @return Topic-safe segment with whitespace collapsed and slashes replaced.
func sanitizeTopicSegment(name string) string {
	segment := strings.TrimSpace(name)
	segment = strings.ReplaceAll(segment, "/", "_")
	return strings.Join(strings.Fields(segment), "_")
}

// @brief Resolves the absolute path to the cached eta_menu.json file.
// @details
// The path is derived relative to the current source file via runtime metadata,
// ensuring cache reads and writes remain stable regardless of current working
// directory when the process is started.
// @return Absolute cache file path, or an error if runtime path lookup fails.
func etaMenuCachePath() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("could not resolve rest package path")
	}
	return filepath.Join(filepath.Dir(file), etaMenuFileName), nil
}

// @brief Loads and parses cached eta_menu.json from the rest package directory.
// @details
// This function reads the cached JSON representation and unmarshals it into the
// EtaMenu structure. It enables startup without immediate network access.
// @param[out] Eta_menu Parsed menu object when load and parse succeed.
// @return Parsed menu structure, or an error if file access or JSON parsing fails.
func loadCachedETAMenu() (EtaMenu, error) {
	cachePath, err := etaMenuCachePath()
	if err != nil {
		return EtaMenu{}, err
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return EtaMenu{}, err
	}

	var menu EtaMenu
	if err := json.Unmarshal(data, &menu); err != nil {
		return EtaMenu{}, err
	}

	return menu, nil
}

// @brief Persists an ETA menu object into the cache file.
// @details
// The menu is serialized with indentation to keep the cached file readable for
// diagnostics. Existing cache content is fully replaced.
// @param[in] menu Parsed or enriched ETA menu to persist.
// @return Error if path resolution, marshaling, or file write fails.
func saveETAMenu(menu EtaMenu) error {
	cachePath, err := etaMenuCachePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(menu, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

// @brief Persists eta_menu.json only when cache file does not already exist.
// @details
// This helper prevents unnecessary overwrite of an existing cache and is used
// for one-time initialization of the cached menu file.
// @param[in] menu Parsed ETA menu to store.
// @return Error if file checks fail or writing new cache fails.
func saveETAMenuIfMissing(menu EtaMenu) error {
	cachePath, err := etaMenuCachePath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(cachePath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	return saveETAMenu(menu)
}

// @brief Loads cached ETA menu or fetches and stores it when cache is missing.
// @details
// The function first attempts to load the local cache. On cache miss or parse
// error it fetches a fresh menu via HTTP and stores it for future calls.
// Non-fatal cache-write errors are logged but do not fail the request.
// @param[in] url Source URL for fetching ETA menu when cache is unavailable.
// @param[out] Eta_menu Parsed menu object loaded from cache or fetched remotely.
// @return Parsed menu structure, or an error if load/fetch fails.
func LoadOrFetchETAMenu(url string) (EtaMenu, error) {
	menu, err := loadCachedETAMenu()
	if err == nil {
		return menu, nil
	}
	if !os.IsNotExist(err) {
		log.Printf("[REST] could not load cached ETA menu, fetching fresh copy: %v", err)
	}

	menu, err = FetchETAMenu(url)
	if err != nil {
		return EtaMenu{}, err
	}

	if err := saveETAMenuIfMissing(menu); err != nil {
		log.Printf("[REST] could not persist ETA menu cache: %v", err)
	} else {
		cachePath, pathErr := etaMenuCachePath()
		if pathErr == nil {
			log.Printf("[REST] ETA menu cached at %s", cachePath)
		}
	}

	return menu, nil
}

// @brief Publishes menu-derived topic mappings where payload value is the URI.
// @details
// For each URI->topic mapping, the function publishes a retained-like semantic
// value flow (topic contains path, payload contains URI) and mirrors the value
// into the in-memory ETA tree. Publication is sorted and throttled to reduce
// bursts on constrained brokers. Failed publishes trigger reconnect wait and a
// single retry.
// @param[in] menu Parsed ETA menu used to generate topic paths.
// @param[in] mqttManager Active MQTT manager used for publish calls.
func publishMenuTopics(menu *EtaMenu, mqttManager *mqtt.Manager) {
	uriTopicMap := BuildURITopicMap(menu)
	topics := make([]string, 0, len(uriTopicMap))
	values := newTopicValueStore(len(uriTopicMap))
	for uri, topicPath := range uriTopicMap {
		topic := "eta/menu/" + topicPath
		topics = append(topics, topic)
		values.set(topic, uri)
	}
	sort.Strings(topics)

	for i, topic := range topics {
		value, ok := values.get(topic)
		if !ok {
			continue
		}

		// Nur publishen wenn sich der Wert geändert hat.
		if prev, ok := publishedValues.get(topic); ok && prev == value {
			continue
		}

		etaTree.set(topic, value)

		if i > 0 && i%menuPublishPauseEvery == 0 {
			time.Sleep(menuPublishPause)
		}

		if err := mqttManager.Publish(topic, value); err != nil {
			log.Printf("[REST] menu publish failed for topic %s: %v", topic, err)
			if !waitForMQTTConnection(mqttManager, mqttReconnectTimeout) {
				return
			}
			if retryErr := mqttManager.Publish(topic, value); retryErr != nil {
				log.Printf("[REST] menu publish retry failed for topic %s: %v", topic, retryErr)
				return
			}
		}
		publishedValues.set(topic, value)
	}
}

// @brief Merges current variable values from varset payload into menu Value fields.
// @details
// This enrichment step copies live value attributes from the current varset
// response into structurally matching menu nodes. The resulting menu snapshot
// then contains both hierarchy metadata and latest values.
// @param[in,out] menu Parsed ETA menu updated in place.
// @param[in] payload Current variable set payload used as value source.
func enrichMenuWithVarset(menu *EtaMenu, payload *Varset_Head) {
	if menu == nil || payload == nil {
		return
	}

	byURI := make(map[string]Varset_Variable, len(payload.Variable.Objects))
	for _, variable := range payload.Variable.Objects {
		byURI[normalizeTopicURI(variable.URI)] = variable
	}

	for i := range menu.Menu.Fub.Objects {
		applyVarsetValues(&menu.Menu.Fub.Objects[i], byURI)
	}
}

// @brief Recursively applies varset values to matching menu objects by normalized URI.
// @details
// For each menu object, the function checks whether a normalized URI exists in
// the lookup map and then copies all relevant value attributes into obj.Value.
// Child objects are processed recursively depth-first.
// @param[in,out] obj Current menu object node updated in place.
// @param[in] byURI Lookup map from normalized URI to varset variable payload.
func applyVarsetValues(obj *Object, byURI map[string]Varset_Variable) {
	if obj == nil {
		return
	}

	if variable, ok := byURI[normalizeTopicURI(obj.URI)]; ok {
		obj.Value.URI = variable.URI
		obj.Value.StrValue = variable.StrValue
		obj.Value.Unit = variable.Unit
		obj.Value.DecPlaces = variable.DecPlaces
		obj.Value.ScaleFactor = variable.ScaleFactor
		obj.Value.AdvTextOffset = variable.AdvTextOffset
		obj.Value.Text = variable.Text
	}

	for i := range obj.Objects {
		applyVarsetValues(&obj.Objects[i], byURI)
	}
}

// @brief Waits for MQTT reconnect until timeout expires.
// @details
// The function polls connection status in short intervals until either a
// connection becomes available or the deadline is reached.
// @param[in] mqttManager Active MQTT manager instance.
// @param[in] timeout Maximum duration to wait for reconnect.
// @return true if a connection is available before timeout, otherwise false.
func waitForMQTTConnection(mqttManager *mqtt.Manager, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if mqttManager.IsConnected() {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return mqttManager.IsConnected()
}

// @brief Fetches and parses the ETA menu XML from the given URL.
// @details
// A timeout-bound HTTP GET request is executed with XML-friendly Accept header.
// Non-success status codes include a truncated body snippet for diagnostics.
// Successful payloads are unmarshaled into EtaMenu.
// @param[in] url Source URL for the ETA menu XML (e.g. http://<host>:<port>/user/menu).
// @param[out] Eta_menu Parsed menu object on success.
// @return Parsed Eta_menu structure or an error if HTTP fetch or XML parse fails.
func FetchETAMenu(url string) (EtaMenu, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return EtaMenu{}, err
	}
	req.Header.Set("Accept", "application/xml, application/json, text/xml, */*")

	resp, err := httpClient.Do(req)
	if err != nil {
		return EtaMenu{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return EtaMenu{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return EtaMenu{}, fmt.Errorf("status %d: %s", resp.StatusCode, snippet)
	}

	var menu EtaMenu
	if err := xml.Unmarshal(body, &menu); err != nil {
		return EtaMenu{}, fmt.Errorf("parse error: %w", err)
	}
	return menu, nil
}

// @brief Fetches ETA menu and creates/populates the target variable set.
// @details
// The routine ensures the target variable set exists and then recursively adds
// all discovered menu object URIs. A warning indicating an already existing set
// is treated as non-fatal to support idempotent initialization.
// @param[in] url Source URL for menu XML.
// @param[in] variableSet Target variable set URL.
// @param[out] menu Destination structure for parsed menu data.
// @return Error if fetching, parsing, or variable set creation fails.
func buildVarSetFromETAMenu(url string, variableSet string, menu *EtaMenu) error {
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

// @brief Sends PUT request to create variable set or append one object URI.
// @details
// The endpoint is built by concatenating the base varset URL with an optional
// object URI suffix. The response body is parsed for ETA error semantics so
// known non-fatal cases can be surfaced as warning text instead of hard errors.
// @param[in] url Base variable set URL.
// @param[in] value Optional object URI suffix to add to the set.
// @param[out] warning Non-fatal API warning text (e.g. set already exists).
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

// @brief Recursively adds all menu object URIs to a target variable set.
// @details
// The function traverses the complete object tree and appends each object URI
// to the target variable set endpoint. Parent nodes are added before children.
// @param[in] varSet Target variable set URL.
// @param[in] in Input object tree.
func addObjectsToVarSet(varSet string, in []Object) {

	for _, obj := range in {
		// Add current node first, then recurse into children.
		createVarSetEntry(varSet, obj.URI)
		if obj.Objects != nil {
			addObjectsToVarSet(varSet, obj.Objects)
		}
	}
}

// @brief Converts a varset URL to its corresponding /user/menu URL.
// @details
// The helper derives the menu endpoint from a known varset endpoint format and
// is primarily used for automatic variable-set creation fallback logic.
// @param[in] varSetURL URL to /user/vars/<name>.
// @return Derived menu URL or an error if format does not match expected pattern.
func deriveMenuURLFromVarSetURL(varSetURL string) (string, error) {
	idx := strings.Index(varSetURL, "/user/vars/")
	if idx == -1 {
		return "", fmt.Errorf("cannot derive menu URL from varset URL: %s", varSetURL)
	}

	return varSetURL[:idx] + "/user/menu", nil
}

// @brief Reads a variable set from URL and unmarshals it into output object.
// @details
// A timeout-bound HTTP GET request is issued against the varset endpoint.
// On 404, the function attempts one-time auto-creation of the missing set by
// deriving the corresponding menu endpoint and rebuilding from menu metadata.
//
// For successful responses, XML payload is unmarshaled into the provided output
// structure if out is non-nil.
// @param[in] url Source URL of the variable set.
// @param[out] out Destination structure for parsed XML response.
// @return Error if request fails, response is non-success, or XML parse fails.
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

			var eta EtaMenu
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

// @brief Requests the configured variable set once and publishes values to MQTT.
// @details
// This function performs one complete acquisition-and-publish cycle:
// 1) load and validate REST config,
// 2) request current varset payload,
// 3) load/fetch menu metadata for URI->name/topic mappings,
// 4) publish menu topics and per-variable scalar fields,
// 5) mirror all published values into the in-memory ETA tree.
//
// The returned payload contains normalized URIs and, when possible, enriched
// display names from the menu structure.
// @param[in] restConfigPath Path to REST JSON configuration.
// @param[in] mqttManager MQTT manager object used for publish calls.
// @param[in] topic Unused compatibility parameter; effective topic derives from menu path.
// @param[out] Varset_Head Fetched and name-enriched variable set payload.
// @return Fetched payload and an error if
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
	uriTopicMap := make(map[string]string)
	menu, err := LoadOrFetchETAMenu(restClient.menuURL())
	if err != nil {
		log.Printf("[REST] could not fetch menu for name mapping: %v", err)
	} else {
		uriNameMap := BuildURINameMap(&menu)
		uriTopicMap = BuildURITopicMap(&menu)
		publishMenuTopics(&menu, mqttManager)
		for i := range payload.Variable.Objects {
			normalizedURI := normalizeTopicURI(payload.Variable.Objects[i].URI)
			if name, ok := uriNameMap[normalizedURI]; ok {
				payload.Variable.Objects[i].Name = name
			}
			payload.Variable.Objects[i].URI = normalizedURI
		}
		enrichMenuWithVarset(&menu, &payload)
		if err := saveETAMenu(menu); err != nil {
			log.Printf("[REST] could not persist enriched ETA menu: %v", err)
		}
	}

	// Publish each variable value to its own topic: "eta/<menu-path>/<field>"
	// Nur Topics mit geändertem Wert werden tatsächlich an den Broker gesendet.
	changed := 0
	for _, v := range payload.Variable.Objects {
		topicPath := v.URI
		if mappedTopic, ok := uriTopicMap[v.URI]; ok {
			topicPath = mappedTopic
		}
		baseTopic := "eta/" + topicPath

		if prev, ok := publishedValues.get(baseTopic); !ok || prev != v.URI {
			etaTree.set(baseTopic, v.URI)
			if err := mqttManager.Publish(baseTopic, v.URI); err != nil {
				log.Printf("[REST] publish failed for topic %s: %v", baseTopic, err)
			} else {
				publishedValues.set(baseTopic, v.URI)
				changed++
			}
		}

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
			if strings.TrimSpace(fieldValue) == "" {
				continue
			}
			topic := baseTopic + "/" + fieldName
			if prev, ok := publishedValues.get(topic); ok && prev == fieldValue {
				continue
			}
			etaTree.set(topic, fieldValue)
			if err := mqttManager.Publish(topic, fieldValue); err != nil {
				log.Printf("[REST] publish failed for topic %s: %v", topic, err)
			} else {
				publishedValues.set(topic, fieldValue)
				changed++
			}
		}
	}

	log.Printf("published %d changed values to MQTT (total variables: %d)", changed, len(payload.Variable.Objects))
	return payload, nil
}

// @brief Polls variable set periodically and publishes it until context cancellation.
// @details
// The loop executes one immediate publish cycle and then repeats at the given
// interval using a ticker. Context cancellation cleanly stops the loop. This
// design allows predictable periodic updates while integrating into caller
// lifecycle management.
// @param[in] ctx Cancellation context controlling lifetime of polling loop.
// @param[in] restConfigPath Path to REST JSON configuration.
// @param[in] mqttManager MQTT manager object used for publish calls.
// @param[in] topic Compatibility topic argument propagated to single-run function.
// @param[in] interval Polling interval. If <= 0, default 60s is used.
// @return Error if configuration is invalid or mqttManager is nil.
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

// @brief Performs a sample GET request against a wallbox variable-set endpoint.
// @details
// This diagnostic helper triggers one manual request flow that can be used
// during development to verify connectivity and response behavior.
// @return No return value. Errors are logged for manual debugging.
func FetchWallboxVarSet() {
	var eta EtaMenu
	// Example call path kept for manual verification/debugging.
	log.Printf("fetching API request...")
	if err := FetchURLResponse("http://192.168.188.99:8080/user/vars/myset1", nil, &eta); err != nil {
		log.Printf("error on API request: %v", err)
		return
	}
	log.Printf("API request completed")
}

// @brief Performs a GET request and optionally copies response body into destination object.
// @details
// The helper executes a timeout-bound HTTP GET and validates status codes.
// When dest is provided as *string or *[]byte, the raw body content is copied
// into that target for caller-side processing.
//
// The payload parameter is kept only for compatibility with historical call
// signatures and is currently unused.
// @param[in] url Source URL to request.
// @param[in] payload Unused placeholder for compatibility with older call sites.
// @param[in,out] dest Optional destination for raw response body (*string or *[]byte).
// @return Error if request fails or HTTP response is not successful.
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
