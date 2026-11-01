# ChickenDoor: Allgemeiner Ablauf

Diese Übersicht beschreibt den gemeinsamen Start-, Persistenz- und Motorlaufpfad. Die einzelnen Bedienmodi sind separat dokumentiert:

- [Manuelle Steuerung](CHICKENDOOR_FLOW_MANUAL.md)
- [Sleep-Schedule](CHICKENDOOR_FLOW_SCHEDULE.md)
- [Testmodus](CHICKENDOOR_FLOW_TEST_MODE.md)

## Start und gemeinsamer Dienst

```mermaid
flowchart TD
    Main[api-go startet MQTT-Manager] --> New[ChickenDoor.New]
    New --> Defaults[Defaults initialisieren]
    Defaults --> Restore[bbolt-Einstellungen, Moduszustand und Historien laden]
    Restore --> Run[ChickenDoor.Run]
    Run --> Selected{Schedule ausgewählt oder aktiv?}
    Selected -- Ja --> Valid{Schedule-Einträge vorhanden und gültig?}
    Valid -- Ja --> Resume[Schedule aktivieren und wakeAt auf jetzt setzen]
    Resume --> Pending{Offener Auftrag oder Schlafzyklus vorhanden?}
    Pending -- Ja --> Keep[Gespeicherten Laufzeitzustand beibehalten]
    Pending -- Nein --> Arm[scheduleSleepPending setzen]
    Keep --> Persist[Wiederaufnahme in bbolt speichern]
    Arm --> Persist
    Persist --> Publish[Mqtt: nano/esp32/schedule_active = true]
    Valid -- Nein --> Loop[1-Sekunden-Dienstloop starten]
    Selected -- Nein --> Loop
    Publish --> Loop

    Loop --> Schedule[scheduleTick]
    Schedule --> Test[testModeTick]
    Test --> Motor[autoStopTick]
    Motor --> Loop
```

Die Weboberfläche lädt Status und UI-State über die API. Dauerhafte Einstellungen werden per `PUT /api/huehnerklappe/ui-state` nach bbolt gespeichert. Der aktive Motorlauf wird beim Backend-Start aus dem aktuellen MQTT-Zustand rekonstruiert, nicht aus bbolt geladen. Ein Wechsel zum Schedule-Tab aktiviert den Schedule im laufenden Betrieb nicht; eine gespeicherte Schedule-Auswahl mit gültigen Einträgen wird beim Backend-Neustart wieder aufgenommen.

## Gemeinsamer Motorlauf und Auto-Stop

Manuelle Steuerung, Schedule und Testmodus verwenden denselben Motorlauf-Tracker. Bei Öffnen oder Schließen publiziert das Backend zunächst die Richtungs-Laufzeit und dann den Befehl an `nano/esp32/engine`. Der Backend-Tick verfolgt den ECU-Zustand über `nano/esp32/engine/set` und die Endschalter.

```mermaid
flowchart TD
    Command[Modus sendet open oder close] --> Runtime[Richtungs-Laufzeit an ECU publizieren]
    Runtime --> Engine[Motorbefehl an nano/esp32/engine]
    Engine --> State[engine/set meldet open oder close]
    State --> Track[autoStopTick erfasst Laufzeit und Endschalter]
    Track --> Terminal{Bewegung beendet?}
    Terminal -- ECU meldet stop oder standby --> Finish[Endzeit aus MQTT-Zeitstempel übernehmen]
    Terminal -- Ziel-Endschalter aktiv --> Finish
    Terminal -- Timeout noch nicht erreicht --> State
    Terminal -- Timeout erreicht --> Stop[nano/esp32/engine = stop publizieren]
    Stop --> Finish
    Finish --> Record[Position und Motorlaufzeit für Status oder Verlauf festhalten]
    Record --> TestSleep{Testmodus-Schlaf ausstehend?}
    TestSleep -- Ja --> Sleep[nächste Test-Schlafzeit publizieren]
    TestSleep -- Nein --> Done[Motorlauf abgeschlossen]
```

Der Timeout verwendet den kleineren Wert aus der richtungsabhängigen Auto-Stop-Zeit und dem gemeinsamen `engineMaxRuntimeSeconds`-Limit. `engine/status` ist MQTT-Subscription-Health, nicht die Motorposition. Endlage und Motorbewegung werden aus `engine/set` und den Endschalter-Themen abgeleitet.