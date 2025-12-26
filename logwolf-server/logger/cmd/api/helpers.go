package main

import (
	"logwolf-toolbox/json"
	"net/http"
)

type jsonResponse struct {
	Error   bool        `json:"error"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (app *Config) readJSON(w http.ResponseWriter, r *http.Request, data interface{}) error {
	return json.ReadJSON(w, r, data)
}

func (app *Config) writeJSON(w http.ResponseWriter, status int, data interface{}, headers ...http.Header) error {
	return json.WriteJSON(w, status, data, headers...)
}

func (app *Config) errorJSON(w http.ResponseWriter, err error, status ...int) error {
	return json.ErrorJSON(w, err, status...)
}
