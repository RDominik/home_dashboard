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
	Objects []Varset_Variable `xml:"object"` // Erste Ebene der Objekte
}

type Varset_Variable struct {
	XMLName       xml.Name `xml:"variable"`
	URI           string   `xml:"uri,attr"`
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

// menuURL builds the menu endpoint URL for the ETA device.
// @return URL to /user/menu.
func (r *RestClient) menuURL() string {
	return r.baseURL() + "/user/menu"
}

// varsetURL builds the configured variable set endpoint URL.
// @return URL to /user/vars/<varset>.
func (r *RestClient) varsetURL() string {
	return fmt.Sprintf("%s/user/vars/%s", r.baseURL(), r.Varset)
}

// BuildVarsetFromConfig loads the REST target from JSON configuration and creates the configured variable set.
// @param configPath Path to the JSON configuration file.
// @return An error if loading the configuration, reading the menu, or creating the variable set fails.
func BuildVarsetFromConfig(configPath string) error {
	// Load target settings from config first.
	client, err := NewRest(configPath)
	if err != nil {
		return err
	}

	// Parse menu and create/fill the configured variable set.
	var eta Eta_menu
	return parseEtaMenuToVarSet(client.menuURL(), client.varsetURL(), &eta)
}

// Parse_eta_menu_to_varSet reads an ETA menu from the given URL and populates a variable set.
// @param url Source URL for the ETA menu XML.
// @param menu Destination object that receives the parsed menu.
// @return An error if the menu cannot be fetched or parsed, or if the variable set cannot be created.
func Parse_eta_menu_to_varSet(url string, menu *Eta_menu) error {
	// Backward-compatible default for legacy callers.
	variableSet := "http://192.168.188.99:8080/user/vars/myset1"
	return parseEtaMenuToVarSet(url, variableSet, menu)
}

// parseEtaMenuToVarSet fetches the ETA menu XML and creates/populates a variable set.
// @param url Source URL for menu XML.
// @param variableSet Target variable set URL.
// @param menu Destination structure for parsed menu data.
// @return An error if fetching, parsing, or creating the variable set fails.
func parseEtaMenuToVarSet(url string, variableSet string, menu *Eta_menu) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Read current menu tree from ETA.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	// Ask primarily for XML (JSON remains fallback).
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
	// Surface non-success responses with a short body snippet.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return fmt.Errorf("status %d: %s", resp.StatusCode, snippet)
	}

	// Parse the full menu into the caller-provided struct.
	err = xml.Unmarshal([]byte(body), menu)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	// Ensure the target variable set exists.
	warning, err := create_variableSet(variableSet, "")
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
	add_variable_to_set(variableSet, menu.Menu.Fub.Objects)

	return nil
}

// create_variableSet issues a PUT to create a variable set or add a single variable path.
// @param url Base variable set URL.
// @param value Optional object URI suffix to add to the set.
// @return Warning text for non-fatal API messages and an error for hard failures.
func create_variableSet(url string, value string) (string, error) {
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

// add_variable_to_set recursively adds all object URIs from menu nodes to a variable set.
// @param varSet Target variable set URL.
// @param in Input object tree.
func add_variable_to_set(varSet string, in []Object) {

	for _, obj := range in {
		// Add current node first, then recurse into children.
		create_variableSet(varSet, obj.URI)s
		if obj.Objects != nil {
			add_variable_to_set(varSet, obj.Objects)
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

// Request_variableSet reads a variable set from the given URL into the provided output structure.
// @param url Source URL of the variable set.
// @param out Destination structure for the parsed XML response.
// @return An error if the request fails or the HTTP response is not successful.
func Request_variableSet(url string, out *Varset_Head) error {
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
			if err := parseEtaMenuToVarSet(menuURL, url, &eta); err != nil {
				return fmt.Errorf("auto-create variable set failed: %w", err)
			}

			log.Printf("variable set created, will be available on next Request_variableSet call")
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

// SendWallboxGet performs a sample GET request against the configured wallbox endpoint.
func SendWallboxGet() {
	var eta Eta_menu
	// Example call path kept for manual verification/debugging.
	log.Printf("fetching API request...")
	if err := PostJSON2("http://192.168.188.99:8080/user/vars/myset1", nil, &eta); err != nil {
		log.Printf("error on API request: %v", err)
		return
	}
	log.Printf("API request completed")
}

// PostJSON2 performs a GET request and optionally stores the raw or parsed response in dest.
// @param url Source URL to request.
// @param payload Unused placeholder for compatibility with older call sites.
// @param dest Optional destination for the response body.
// @return An error if the request fails or the HTTP response is not successful.
func PostJSON2(url string, payload any, dest any) error {
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
