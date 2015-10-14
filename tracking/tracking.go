package tracking

import (
	"fmt"
	"strings"
	"time"

	"github.com/marcusolsson/goddd/cargo"
)

type Service interface {
	Track(id string) (Cargo, error)
}

type service struct {
	cargos         cargo.Repository
	handlingEvents cargo.HandlingEventRepository
}

func (s *service) Track(id string) (Cargo, error) {
	c, err := s.cargos.Find(cargo.TrackingID(id))
	if err != nil {
		return Cargo{}, err
	}
	return assemble(c, s.handlingEvents), nil
}

func NewService() Service {
	return &service{}
}

type Cargo struct {
	TrackingID           string    `json:"trackingId"`
	StatusText           string    `json:"statusText"`
	Origin               string    `json:"origin"`
	Destination          string    `json:"destination"`
	ETA                  time.Time `json:"eta"`
	NextExpectedActivity string    `json:"nextExpectedActivity"`
	Misrouted            bool      `json:"misrouted"`
	Routed               bool      `json:"routed"`
	ArrivalDeadline      time.Time `json:"arrivalDeadline"`
	Events               []Event   `json:"events"`
}

type Event struct {
	Description string `json:"description"`
	Expected    bool   `json:"expected"`
}

func assemble(c cargo.Cargo, her cargo.HandlingEventRepository) Cargo {
	return Cargo{
		TrackingID:           string(c.TrackingID),
		Origin:               string(c.Origin),
		Destination:          string(c.RouteSpecification.Destination),
		ETA:                  c.Delivery.ETA,
		NextExpectedActivity: nextExpectedActivity(c),
		ArrivalDeadline:      c.RouteSpecification.ArrivalDeadline,
		StatusText:           assembleStatusText(c),
		Events:               assembleEvents(c, her),
	}
}

func nextExpectedActivity(c cargo.Cargo) string {
	a := c.Delivery.NextExpectedActivity
	prefix := "Next expected activity is to"

	switch a.Type {
	case cargo.Load:
		return fmt.Sprintf("%s %s cargo onto voyage %s in %s.", prefix, strings.ToLower(a.Type.String()), a.VoyageNumber, a.Location)
	case cargo.Unload:
		return fmt.Sprintf("%s %s cargo off of voyage %s in %s.", prefix, strings.ToLower(a.Type.String()), a.VoyageNumber, a.Location)
	case cargo.NotHandled:
		return "There are currently no expected activities for this cargo."
	}

	return fmt.Sprintf("%s %s cargo in %s.", prefix, strings.ToLower(a.Type.String()), a.Location)
}

func assembleStatusText(c cargo.Cargo) string {
	switch c.Delivery.TransportStatus {
	case cargo.NotReceived:
		return "Not received"
	case cargo.InPort:
		return fmt.Sprintf("In port %s", c.Delivery.LastKnownLocation)
	case cargo.OnboardCarrier:
		return fmt.Sprintf("Onboard voyage %s", c.Delivery.CurrentVoyage)
	case cargo.Claimed:
		return "Claimed"
	default:
		return "Unknown"
	}
}

func assembleEvents(c cargo.Cargo, r cargo.HandlingEventRepository) []Event {
	h := r.QueryHandlingHistory(c.TrackingID)
	events := make([]Event, len(h.HandlingEvents))
	for i, e := range h.HandlingEvents {
		var description string

		switch e.Activity.Type {
		case cargo.NotHandled:
			description = "Cargo has not yet been received."
		case cargo.Receive:
			description = fmt.Sprintf("Received in %s, at %s", e.Activity.Location, time.Now().Format(time.RFC3339))
		case cargo.Load:
			description = fmt.Sprintf("Loaded onto voyage %s in %s, at %s.", e.Activity.VoyageNumber, e.Activity.Location, time.Now().Format(time.RFC3339))
		case cargo.Unload:
			description = fmt.Sprintf("Unloaded off voyage %s in %s, at %s.", e.Activity.VoyageNumber, e.Activity.Location, time.Now().Format(time.RFC3339))
		case cargo.Claim:
			description = fmt.Sprintf("Claimed in %s, at %s.", e.Activity.Location, time.Now().Format(time.RFC3339))
		case cargo.Customs:
			description = fmt.Sprintf("Cleared customs in %s, at %s.", e.Activity.Location, time.Now().Format(time.RFC3339))
		default:
			description = "[Unknown status]"
		}

		events[i] = Event{Description: description, Expected: c.Itinerary.IsExpected(e)}
	}

	return events
}
