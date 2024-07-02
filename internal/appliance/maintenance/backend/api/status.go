package api

import (
	"fmt"
	"net/http"
)

type Status struct {
	Services []*Service `json:"services"`
}

type StatusRequest struct {
	Healthy bool `json:"healthy"`
}

type Service struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Message string `json:"message"`
}

var serviceStatus []*Service

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received request for status")

	result := Status{
		Services: serviceStatus,
	}

	if installError == "" {
		installTasks = progressTasks(installTasks)
	}

	fmt.Println("Sending current status", result)
	sendJson(w, result)
}

func SetStatusHandlerForTesting(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received request to set status")

	var request StatusRequest
	receiveJson(w, r, &request)

	fmt.Println("Setting health to", request.Healthy)
	serviceStatus = serviceList(request.Healthy)
}

func serviceList(healthy bool) []*Service {
	var result []*Service = []*Service{
		{
			Name:    "The Operator",
			Healthy: true,
		},
		{
			Name:    "GraphQL API",
			Healthy: healthy,
			Message: "API is crashing",
		},
		{
			Name:    "Git Service",
			Healthy: true,
		},
		{
			Name:    "Web Frontend",
			Healthy: true,
		},
		{
			Name:    "Upgrader",
			Healthy: healthy,
			Message: "Cannot download Docker image",
		},
	}

	if healthy {
		for _, s := range result {
			s.Message = "OK"
		}
	}

	return result
}
