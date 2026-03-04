package api

import "metadata-service/internal/model"

type SearchResponse struct {
	Works []model.Work `json:"works"`
}

type WorkResponse struct {
	Work model.Work `json:"work"`
}

type EditionResponse struct {
	Edition model.Edition `json:"edition"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
