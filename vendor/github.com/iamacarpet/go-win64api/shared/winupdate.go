package shared

import (
	"time"
)

type WindowsUpdate struct {
	UpdatesReq    bool                    `json:"required"`
	NumUpdates    int                     `json:"number"`
	UpdateHistory []*WindowsUpdateHistory `json:"history"`
}

type WindowsUpdateHistory struct {
	EventDate  time.Time `json:"eventDate"`
	Status     string    `json:"status"`
	UpdateName string    `json:"updateName"`
}
