package weather

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Coords holds latitude and longitude.
type Coords struct {
	Lat float64
	Lon float64
}

// Built-in city coordinates for major global cities.
var cityCoords = map[string]Coords{
	"Shanghai":      {Lat: 31.22, Lon: 121.46},
	"Beijing":       {Lat: 39.90, Lon: 116.41},
	"Tokyo":         {Lat: 35.68, Lon: 139.76},
	"Seoul":         {Lat: 37.57, Lon: 126.98},
	"Singapore":     {Lat: 1.35, Lon: 103.82},
	"Sydney":        {Lat: -33.87, Lon: 151.21},
	"London":        {Lat: 51.51, Lon: -0.13},
	"Paris":         {Lat: 48.85, Lon: 2.35},
	"Berlin":        {Lat: 52.52, Lon: 13.41},
	"New York":      {Lat: 40.71, Lon: -74.01},
	"San Francisco": {Lat: 37.77, Lon: -122.42},
	"Los Angeles":   {Lat: 34.05, Lon: -118.24},
	"Chicago":       {Lat: 41.88, Lon: -87.63},
	"Toronto":       {Lat: 43.65, Lon: -79.38},
	"Dubai":         {Lat: 25.20, Lon: 55.27},
	"Mumbai":        {Lat: 19.08, Lon: 72.88},
	"Bangkok":       {Lat: 13.76, Lon: 100.50},
	"Hong Kong":     {Lat: 22.32, Lon: 114.17},
	"São Paulo":     {Lat: -23.55, Lon: -46.63},
	"Mexico City":   {Lat: 19.43, Lon: -99.13},
	"Moscow":        {Lat: 55.76, Lon: 37.62},
	"Cairo":         {Lat: 30.04, Lon: 31.24},
	"Istanbul":      {Lat: 41.01, Lon: 28.98},
	"Jakarta":       {Lat: -6.21, Lon: 106.85},
}

// Info holds weather display data.
type Info struct {
	City string
	Temp int
	Icon string
}

// wmoToEmoji maps Open-Meteo WMO weather codes to emoji icons.
func wmoToEmoji(code int) string {
	switch code {
	case 0:
		return "☀️"
	case 1, 2, 3:
		return "⛅"
	case 45, 48:
		return "🌫️"
	case 51, 53, 55, 56, 57, 61, 63, 65, 66, 67:
		return "🌧️"
	case 71, 73, 75, 77, 85, 86:
		return "🌨️"
	case 95, 96, 99:
		return "⛈️"
	default:
		return "🌡️"
	}
}

// Get fetches current weather for the given city.
// Falls back to Shanghai if the city is not in the built-in mapping.
func Get(city string) (*Info, error) {
	coords, ok := cityCoords[city]
	if !ok {
		// Try case-insensitive match.
		for name, c := range cityCoords {
			if strings.EqualFold(name, city) {
				coords = c
				city = name
				ok = true
				break
			}
		}
		if !ok {
			return nil, fmt.Errorf("unknown city: %s (supported: Shanghai, Beijing, Tokyo, New York, London, ...)", city)
		}
	}

	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.2f&longitude=%.2f&current_weather=true",
		coords.Lat, coords.Lon,
	)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API returned %d", resp.StatusCode)
	}

	var payload struct {
		CurrentWeather struct {
			Temperature   float64 `json:"temperature"`
			WeatherCode   int     `json:"weathercode"`
		} `json:"current_weather"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	return &Info{
		City: city,
		Temp: int(payload.CurrentWeather.Temperature),
		Icon: wmoToEmoji(payload.CurrentWeather.WeatherCode),
	}, nil
}
