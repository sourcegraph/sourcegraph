package service

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type WeatherService struct{}

func (s *WeatherService) GetCurrentWeather(ctx context.Context, location string) (*WeatherResponse, error) {
	description := "It's cloudy :("

	switch location {
	case "Ravenholm":
		// Let's pretend this location is not real to demonstrate error handling.
		return nil, &InvalidPlaceError{Place: location, Message: "We don't go to Ravenholm."}
	case "Black Mesa":
		return nil, &SensorOfflineError{SensorId: "anomalous-materials", Message: "Sensor is offline. A resonance cascade is imminent."}
	case "Philadelphia":
		description = "It's always sunny!"
	}

	// Now that we know that we have a "valid" location, we can safely return the response.
	response := &WeatherResponse{
		Description: description,
		Temperature: &Temperature{
			Value: 25.0,
			Unit:  CELSIUS,
		},
	}

	return response, nil
}

func (s *WeatherService) SubscribeWeatherAlerts(ctx context.Context, region string, onAlert func(a *WeatherAlert) error) error {
	for range 5 {
		select {
		case <-ctx.Done(): // The client either explicitly canceled the operation or the context timed out.
			return ctx.Err()

		case <-time.After(2 * time.Second): // Wait 2 seconds between the fake alerts we're sending.
			err := onAlert(&WeatherAlert{Alert: "Severe weather alert!"})
			if err != nil {
				return errors.Wrap(err, "failed to send alert")
			}
		}
	}

	return nil
}

func (s *WeatherService) StoreSensorData(ctx context.Context, data *SensorData) error {
	// Imaginary floppy drive starts spinning.

	return nil
}

func (s *WeatherService) StoreWeatherPhoto(r io.Reader) error {
	_, err := io.Copy(io.Discard, r)
	if err != nil {
		return errors.Wrap(err, "failed to copy photo")
	}

	return nil
}

type SensorData struct {
	SensorId    string
	Temperature *Temperature
	Humidity    float64
}

type WeatherAlert struct {
	Alert string
}

type WeatherResponse struct {
	Description string
	Temperature *Temperature
}

type InvalidPlaceError struct {
	Place   string
	Message string
}

func (e *InvalidPlaceError) Error() string {
	return fmt.Sprintf("invalid place: %s", e.Place)
}

type SensorOfflineError struct {
	SensorId string
	Message  string
}

func (e *SensorOfflineError) Error() string {
	return fmt.Sprintf("sensor %s is offline: %s", e.SensorId, e.Message)
}

type TemperatureUnit int

const (
	UNSPECIFIED TemperatureUnit = iota
	CELSIUS
	FAHRENHEIT
	KELVIN
)

type Temperature struct {
	Value float64
	Unit  TemperatureUnit
}
