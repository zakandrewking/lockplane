package shadow

import (
	"encoding/json"
	"errors"
	"os"
	"time"
)

const ReservationFile = ".lockplane-shadow.json"

// Reservation tracks an active shadow DB prep session.
type Reservation struct {
	Environment  string    `json:"environment"`
	ShadowURL    string    `json:"shadow_url"`
	ShadowSchema string    `json:"shadow_schema,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// LoadReservation returns the current reservation if it exists.
func LoadReservation() (*Reservation, error) {
	data, err := os.ReadFile(ReservationFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var res Reservation
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// SaveReservation persists the given reservation.
func SaveReservation(res *Reservation) error {
	if res == nil {
		return errors.New("nil reservation")
	}
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	tmpFile := ReservationFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpFile, ReservationFile)
}

// ClearReservation removes the reservation file.
func ClearReservation() error {
	if err := os.Remove(ReservationFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
