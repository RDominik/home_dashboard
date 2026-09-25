# Weather API integration

This document describes which external weather APIs the application calls, how their data is used, and which local API exposes the normalized result. Keep this document current whenever a provider, endpoint, query parameter, payload, retention rule, or weather MQTT topic changes.

## Providers and responsibilities

### Weather Underground / Weather Company Personal Weather Station

- **Purpose:** Read the configured personal weather station's current observation and its station coordinates.
- **Endpoint:** `GET https://api.weather.com/v2/pws/observations/current`
- **Authentication:** The configured Weather Underground API key is sent as `apiKey`.
- **Required query parameters:** `stationId`, `format=json`, `units=m|e`, `numericPrecision=decimal`, and `apiKey`.
- **Used values:** Station name/ID, observation timestamp, current measurements, latitude, longitude, and elevation.
- **Not used for forecast:** The configured key in this installation is not authorized for the Weather Company hourly forecast product. Forecast requests must not be sent to that product; it responds with HTTP 401 `apikey is not authorized for this product`.

#### Personal Weather Station response data used by the application

The endpoint returns an `observations` array. The service uses its first entry and decodes the following provider fields. Values under `metric` or `imperial` are chosen according to the saved `units` setting.

| Provider JSON field | Normalized status field | Meaning / conversion |
| --- | --- | --- |
| `stationID` | `observation.stationId` | Personal Weather Station identifier. |
| `neighborhood` | `observation.stationName` | Provider's station/neighborhood display name. |
| `obsTimeUtc` | `observation.observedAt` | Time of the station observation in UTC. |
| `metric.temp` / `imperial.temp` | `observation.temperature` | Current air temperature; unit `°C` or `°F`. |
| `metric.feelsLike` / `imperial.feelsLike` | `observation.feelsLike` | Feels-like temperature; unit `°C` or `°F`. |
| `humidity` | `observation.humidity` | Relative humidity in percent. |
| `winddirCardinal` | `observation.windDirection` | Cardinal wind direction text. |
| `metric.windSpeed` / `imperial.windSpeed` | `observation.windSpeed` | Wind speed; unit `km/h` or `mph`. |
| `metric.windGust` / `imperial.windGust` | `observation.windGust` | Wind gust speed; unit `km/h` or `mph`. |
| `metric.pressure` / `imperial.pressure` | `observation.pressure` | Station pressure; unit `hPa` or `inHg`. |
| `metric.dewpt` / `imperial.dewpt` | `observation.dewPoint` | Dew-point temperature; unit `°C` or `°F`. |
| `metric.precipRate` / `imperial.precipRate` | `observation.precipRate` | Current precipitation rate; unit `mm/h` or `in/h`. |
| `metric.precipTotal` / `imperial.precipTotal` | `observation.precipTotal` | Accumulated precipitation; unit `mm` or `in`. |
| `uv` | `observation.uv` | Ultraviolet index. |
| `solarRadiation` | `observation.solarRadiation` | Solar irradiance in W/m². |
| `lat` / `lon` | `observation.latitude` / `observation.longitude` | Station coordinates; passed to Open-Meteo as forecast coordinates. |
| `elev` | `observation.elevation` | Station elevation in meters. |
| Backend refresh time | `observation.updatedAt` | UTC timestamp when this backend refresh completed. |

The PWS response's numeric `winddir` bearing is decoded but not currently exposed; the UI uses the provider's `winddirCardinal` value instead. Other fields returned by the provider but not listed above are currently ignored by this application.

### Open-Meteo Forecast API

- **Purpose:** Provide the 24-hour hourly forecast and sunrise/sunset for the next three local calendar days, without requiring the Weather Company paid forecast product.
- **Endpoint:** `GET https://api.open-meteo.com/v1/forecast`
- **Authentication:** No API key is sent for the public API usage described here.
- **Coordinates:** `latitude` and `longitude` come from the current Personal Weather Station observation.
- **Time range and timezone:** `forecast_hours=24`, `forecast_days=3`, and `timezone=auto` return local timestamps resolved for the coordinates.
- **Requested hourly variables:** `temperature_2m`, `apparent_temperature`, `precipitation_probability`, `precipitation`, `cloud_cover`, `dew_point_2m`, `relative_humidity_2m`, `wind_speed_10m`, `wind_direction_10m`, `pressure_msl`, and `weather_code`.
- **Requested daily variables:** `sunrise` and `sunset`.
- **Units:** Temperature is requested in Celsius or Fahrenheit; wind in km/h or mph; precipitation in mm or inches. Pressure is supplied in hPa and converted to inHg for imperial display. Wind direction degrees are converted to a German 16-point compass direction. WMO weather codes are mapped to German descriptions.
- **Failure behavior:** Forecast failures are returned separately as `forecastError`; the last successful forecast remains cached. A failed forecast does not invalidate the current station observation.
- **Usage terms:** The public endpoint is subject to Open-Meteo's current terms and fair-use policy. The documented free API is for non-commercial use and under 10,000 daily API calls. Commercial use or higher volume requires an appropriate plan. Review [Open-Meteo API documentation](https://open-meteo.com/en/docs) and [terms](https://open-meteo.com/en/terms) before deployment.

#### Open-Meteo request and data-element mapping

The backend makes one forecast request per successful station refresh. `latitude` and `longitude` are rounded to six decimal places. The local timezone is selected with `timezone=auto`; timestamps are kept in that timezone as supplied by Open-Meteo.

| Open-Meteo query parameter | Current value | Purpose |
| --- | --- | --- |
| `latitude`, `longitude` | Station coordinates | Select forecast grid cell. |
| `hourly` | `temperature_2m,apparent_temperature,precipitation_probability,precipitation,cloud_cover,dew_point_2m,relative_humidity_2m,wind_speed_10m,wind_direction_10m,pressure_msl,weather_code` | Select hourly elements returned in parallel arrays. |
| `daily` | `sunrise,sunset` | Request solar times for each local calendar day. |
| `forecast_hours` | `24` | Limit hourly series to the next 24 time points (current-hour aligned). |
| `forecast_days` | `3` | Request daily values for today and the next two local dates. |
| `timezone` | `auto` | Resolve location timezone and return local timestamps. |
| `temperature_unit` | `celsius` or `fahrenheit` | Match the UI's metric/imperial temperature setting. |
| `wind_speed_unit` | `kmh` or `mph` | Match the UI's metric/imperial wind setting. |
| `precipitation_unit` | `mm` or `inch` | Match the UI's metric/imperial precipitation setting. |

| Open-Meteo response element | Local field | Meaning / handling |
| --- | --- | --- |
| `hourly.time` | `forecast[].validTime` | Local ISO-8601 date and hour for the forecast row. |
| `hourly.temperature_2m` | `forecast[].temperature` | Air temperature at 2 m; requested temperature unit. |
| `hourly.apparent_temperature` | `forecast[].feelsLike` | Apparent/feels-like temperature; requested temperature unit. |
| `hourly.precipitation_probability` | `forecast[].precipChance` | Probability of measurable precipitation, percent. |
| `hourly.precipitation` | `forecast[].precipAmount` | Precipitation sum for the preceding hour; mm or inch. |
| `hourly.cloud_cover` | `forecast[].cloudCover` | Total cloud cover, percent. |
| `hourly.dew_point_2m` | `forecast[].dewPoint` | Dew-point temperature at 2 m; requested temperature unit. |
| `hourly.relative_humidity_2m` | `forecast[].humidity` | Relative humidity at 2 m, percent. |
| `hourly.wind_speed_10m` | `forecast[].windSpeed` | Wind speed at 10 m; km/h or mph. |
| `hourly.wind_direction_10m` | `forecast[].windDirection` | Bearing in degrees converted to one of 16 compass labels (`N`, `NNO`, …, `NNW`). |
| `hourly.pressure_msl` | `forecast[].pressure` | Mean-sea-level pressure; hPa, converted to inHg for imperial display. |
| `hourly.weather_code` | `forecast[].condition` | WMO code mapped to a German phrase (e.g. clear, rain, fog, snow, thunderstorm). |
| `daily.time` | `sunTimes[].date` | Local calendar date. |
| `daily.sunrise` | `sunTimes[].sunrise` | Local ISO-8601 sunrise timestamp. |
| `daily.sunset` | `sunTimes[].sunset` | Local ISO-8601 sunset timestamp. |

The status payload also provides unit labels per forecast row: `temperatureUnit`, `windUnit`, `pressureUnit`, and `precipUnit`. Wind directions and weather-code descriptions are normalized by the backend; raw numeric bearings and WMO codes are not exposed in the local forecast contract.

## Local API contract

All local endpoints are served by the Go REST service. The current observation and forecast are cached in bbolt at `data/weather.db` and returned from the status endpoint.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/weather/status` | Read settings, current observation, hourly forecast, three-day sun times, and any current/forecast errors. |
| `GET` | `/api/weather/settings` | Read normalized station and polling settings. |
| `PUT` | `/api/weather/settings` | Persist settings; a successful save starts an asynchronous refresh. |
| `POST` | `/api/weather/refresh` | Request an asynchronous station observation and Open-Meteo forecast refresh. |

The status response includes:

- `observation`: normalized current Personal Weather Station data.
- `forecast`: up to 24 hourly records, with local ISO time, German condition text, measurements, and display units.
- `sunTimes`: up to three records with `date`, local `sunrise`, and local `sunset` values.
- `error`: current-station request error.
- `forecastError`: Open-Meteo request, decoding, or validation error. A Weather Company forecast 401 should not occur because that forecast endpoint is not used.
- `lastFetchAt` and `configured`.

Forecast records are persisted alongside the observation. A forecast error retains the last successful forecast and sunrise/sunset cache.

## MQTT publication

After a successful Open-Meteo forecast refresh, the backend publishes the three-day sunrise/sunset array as a **retained JSON message** to `nano/esp32/sun-times`. The retained flag lets a sleeping ESP32 receive the most recently stored values after reconnecting. The backend also republishes the persisted snapshot on service startup when one exists.

Example payload shape:

```json
[
  { "date": "2026-09-25", "sunrise": "2026-09-25T07:03", "sunset": "2026-09-25T19:05" },
  { "date": "2026-09-26", "sunrise": "2026-09-26T07:05", "sunset": "2026-09-26T19:02" },
  { "date": "2026-09-27", "sunrise": "2026-09-27T07:06", "sunset": "2026-09-27T19:00" }
]
```

The ESP32 firmware must subscribe to this topic and parse the JSON array. Firmware compatibility is outside this Go/Web UI repository.

## Operational notes

- Weather Underground API-key and station settings remain server-side and are persisted by the Go service.
- The UI refresh interval applies to the current observation and the combined Open-Meteo forecast refresh.
- Current station readings and model forecasts have different data origins and may differ slightly; forecast coordinates are the station's reported coordinates.
- API field definitions and availability can change. Validate the actual provider response and terms before adding or removing requested variables.
