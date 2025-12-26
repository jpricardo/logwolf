package main

import (
	"logwolf-toolbox/data"
	"net/http"
)

func (app *Config) WriteLog(w http.ResponseWriter, r *http.Request) {
	var requestPayload data.JSONLogPayload
	_ = app.readJSON(w, r, requestPayload)

	event := data.LogEntry{Name: requestPayload.Name, Data: requestPayload.Data}

	err := app.Models.LogEntry.Insert(event)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	response := jsonResponse{
		Error:   false,
		Message: "Logged!",
	}

	app.writeJSON(w, http.StatusAccepted, response)
}
