# ChickenDoor: Manuelle Steuerung

Die manuelle Ansicht bietet Öffnen, Schließen, Stop sowie Schlafen für eine Dauer oder bis zu einer Uhrzeit.

```mermaid
flowchart TD
    UI[Manuelle Registerkarte öffnen] --> Tab{Manuelle Steuerung aktivieren?}
    Tab -- Nein, nur Tab ansehen --> NoChange[Kein Moduswechsel]
    Tab -- Ja --> Disable[Schedule über sleep-schedule active=false deaktivieren]
    Disable --> SaveMode[controlMode=manual speichern]

    UI --> Action{Manuelle Aktion}
    Action -- Öffnen oder Schließen --> SendMove[PUT /api/huehnerklappe/set mit engine=open oder close]
    SendMove --> Validate[Backend prüft erlaubten Befehl]
    Validate --> Runtime[Richtungs-Laufzeit an nano/esp32/engineMaxRuntime publizieren]
    Runtime --> Engine[Bewegung an nano/esp32/engine publizieren]
    Engine --> Track[Backend startet Motorlauf-Tracking]
    Track --> Monitor[Gemeinsamer Auto-Stop- und Endschalterpfad]

    Action -- Stop --> SendStop[PUT /api/huehnerklappe/set mit engine=stop]
    SendStop --> StopTopic[nano/esp32/engine = stop publizieren]
    StopTopic --> Clear[Motorlauf-Tracking beenden]

    Action -- Schlafdauer --> Duration[Sleep-Dauer in Sekunden eingeben]
    Action -- Schlafen bis Uhrzeit --> Time[Uhrzeit wählen und verbleibende Sekunden berechnen]
    Duration --> Sleep[PUT /set mit engine/sleep]
    Time --> Sleep
    Sleep --> Convert[Backend multipliziert Sekunden mit 1000]
    Convert --> SleepTopic[nano/esp32/sleepms publizieren]
```

Die Registerkarte allein aktiviert den manuellen Modus nicht. Dafür ist die Schaltfläche „Manuelle Steuerung aktivieren“ erforderlich; sie deaktiviert den Schedule explizit. Der gemeinsame Motorlauf-/Auto-Stop-Pfad ist in der [allgemeinen Übersicht](CHICKENDOOR_FLOW_GENERAL.md) beschrieben.