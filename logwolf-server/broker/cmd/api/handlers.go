package main

import (
	"encoding/json"
	"logwolf-toolbox/event"
	"net/http"
)

type LogPayload struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

func (app *Config) HandleSubmission(w http.ResponseWriter, r *http.Request) {
	var payload LogPayload

	err := app.readJSON(w, r, &payload)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	// Push to queue
	evp := event.Payload{Action: "log", Log: event.LogPayload(payload)}

	emitter, err := event.NewEmitter(app.Rabbit)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	j, err := json.MarshalIndent(&evp, "", "\t")
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	err = emitter.Push(string(j), "log.INFO")
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusAccepted, jsonResponse{Error: false, Message: "OK!"})
}
