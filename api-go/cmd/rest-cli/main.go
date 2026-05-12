package main

import (
	"encoding/json"
	"fmt"

	restpkg "webgui-api/rest"
)

func parseValue(s string) any {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return v
	}
	return s
}

func main() {
	print("start of rest-cli\n")
	configPath := "rest/rest_config.json"
	if err := restpkg.BuildVarSetFromConfig(configPath); err != nil {
		fmt.Println("Fehler beim Erzeugen des Variablen-Sets:", err)
		return
	}

	cfg, err := restpkg.NewRest(configPath)
	if err != nil {
		fmt.Println("Fehler beim Laden der REST-Konfiguration:", err)
		return
	}

	var eta2 restpkg.Varset_Head
	varsetURL := fmt.Sprintf("http://%s:%d/user/vars/%s", cfg.Client, cfg.Port, cfg.Varset)
	restpkg.RequestVarSet(varsetURL, &eta2)
	// restpkg.Var_put("http://192.168.188.99:8080/user/vars/myset1", "")
	// restpkg.Var_put("http://192.168.188.99:8080/user/vars/myset1", "/24/10561/0/11031/2012")
	// restpkg.Var_put("http://192.168.188.99:8080/user/vars/myset1", "/24/10561/0/11031/2013")
	// restpkg.Var_put("http://192.168.188.99:8080/user/vars/myset1", "/24/10561/0/0/10990")
	// restpkg.FetchWallboxVarSet()

}
