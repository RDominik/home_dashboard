# ChickenDoor: Sleep-Schedule

Der Schedule schläft bis zum nächsten konfigurierten Zeitstempel, wertet den Aufwachzeitpunkt aus und führt die hinterlegte Aktion aus.

```mermaid
flowchart TD
    Configure[Zeitstempel, Aktionen und Wachzeit konfigurieren] --> Activate[Schedule ausdrücklich aktivieren]
    Activate --> API[PUT /api/huehnerklappe/sleep-schedule mit active=true]
    API --> Persist[Werte und Aktivierung übernehmen und in bbolt speichern]
    Persist --> Topic[nano/esp32/schedule_active = true publizieren]
    Topic --> Arm[Schedule-Zyklus anstoßen]

    Restore[Backend-Neustart mit controlMode=schedule] --> RestoreValid{Mindestens ein gültiger Eintrag?}
    RestoreValid -- Ja --> Resume[Schedule aktivieren, wakeAt setzen und Laufzeitstatus erhalten]
    Resume --> Arm
    RestoreValid -- Nein --> Idle[Schedule nicht starten]

    Arm --> Tick[scheduleTick, einmal pro Sekunde]
    Tick --> Ready{Schedule aktiv, Deadline erreicht und Controller online?}
    Ready -- Nein --> Tick
    Ready -- Ja --> Sleeping{Controller schläft oder ist offline?}
    Sleeping -- Ja --> Tick
    Sleeping -- Nein --> Pending{Schlafzyklus wartet?}

    Pending -- Ja --> Next[Nächsten Schedule-Eintrag ermitteln]
    Next --> SleepCmd[nano/esp32/sleepms bis zum Zeitstempel publizieren]
    SleepCmd --> History[Sleep-Befehl und Verlauf in bbolt speichern]
    History --> Wake[ESP32 wacht auf und meldet online]
    Wake --> Timing{Aufwachzeit relativ zum Termin?}
    Timing -- Höchstens 5 s zu früh --> Wait[Online bis zum geplanten Termin warten]
    Timing -- Mehr als 5 s zu früh --> RetrySleep[Aktion auslassen und erneut schlafen]
    Timing -- Pünktlich oder später --> Resolve[Gespeicherte Aktion auflösen]
    Wait --> Resolve
    RetrySleep --> SleepCmd

    Pending -- Nein --> Resolve
    Resolve --> Target{Ziel-Endschalter bereits aktiv?}
    Target -- Ja --> Skip[Motorbefehl überspringen]
    Target -- Nein --> Action[Motorlaufzeit und engine-Aktion publizieren]
    Skip --> Awake[Konfigurierte Wachzeit abwarten]
    Action --> Awake
    Awake --> Pending
```

Die Schedule-Aktivierung bleibt explizit: Ein Tab-Wechsel allein aktiviert sie nicht. Beim Neustart wird eine gespeicherte Schedule-Auswahl mit gültigen Einträgen wieder aufgenommen. Das ECU-Gesundheitsthema `engine/status` ist nicht die Motorposition; Bewegung wird über `engine/set`, die Position über die Endschalter erkannt. Der Verlauf enthält maximal 20 Einträge.

Der gemeinsame Motorlauf-/Auto-Stop-Pfad ist in der [allgemeinen Übersicht](CHICKENDOOR_FLOW_GENERAL.md) beschrieben.