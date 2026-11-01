# ChickenDoor: Testmodus

Der Testmodus führt innerhalb des täglichen Zeitfensters in einem einstellbaren Intervall abwechselnd Öffnen und Schließen aus. Ein aktiver Timestamp-Schedule pausiert den Testmodus.

```mermaid
flowchart TD
    Settings[Testmodus aktivieren und Intervall, Zeitfenster, Maximalwachzeit speichern] --> Tick[testModeTick, einmal pro Sekunde]
    Tick --> Active{Testmodus aktiv und Schedule inaktiv?}
    Active -- Nein --> Wait[Auf nächsten Tick warten]
    Active -- Ja --> Window{Innerhalb des täglichen Zeitfensters?}
    Window -- Nein --> Clear[Intervall-Deadline löschen]
    Clear --> Tick
    Window -- Ja --> Busy{Motor läuft oder Test-Schlaf ausstehend?}
    Busy -- Ja --> Wait
    Busy -- Nein --> Deadline{Aktionsdeadline gesetzt und erreicht?}
    Deadline -- Nein, noch keine Deadline --> Arm[Erstes Intervall vollständig abwarten und Deadline speichern]
    Arm --> Tick
    Deadline -- Nein, noch nicht fällig --> Tick
    Deadline -- Ja --> Online{Controller online und nicht schlafend?}
    Online -- Nein --> Wait
    Online -- Ja --> Direction[Nächste Richtung wählen; zuerst open, danach abwechselnd]
    Direction --> Runtime[Richtungs-Laufzeit an ECU publizieren]
    Runtime --> Move[nano/esp32/engine = open oder close publizieren]
    Move --> Record[Motorstatus, nächste Deadline und Testverlauf speichern]
    Record --> Motor[Motorlauf]
    Motor --> Common[Gemeinsame Auto-Stop- und Endschalterüberwachung]
    Common --> Done{Bewegung beendet?}
    Done -- Nein --> Tick
    Done -- Ja --> Awake[Maximalwachzeit abwarten, mindestens Motorlaufzeit]
    Awake --> Sleep[Restzeit bis zur nächsten Aktionsdeadline berechnen]
    Sleep --> SleepTopic[nano/esp32/sleepms publizieren]
    SleepTopic --> Tick
```

Ist die Aktionsdeadline erreicht, während der Controller nicht verfügbar ist, bleibt der Auftrag fällig und kann nach der Wiederverbindung einmal ausgeführt werden. Die Maximalwachzeit verlängert das Aktionsintervall nicht: Der Schlaf deckt nur die verbleibende Zeit bis zur nächsten Deadline ab; mindestens eine Sekunde Schlaf bleibt bestehen.

Der Testmodus setzt oder publiziert nicht `nano/esp32/schedule_active`. Die gemeinsame Motorlauf-/Auto-Stop-Logik ist in der [allgemeinen Übersicht](CHICKENDOOR_FLOW_GENERAL.md) beschrieben.