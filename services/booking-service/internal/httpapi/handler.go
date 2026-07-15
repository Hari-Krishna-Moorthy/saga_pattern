package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Hari-Krishna-Moorthy/ride-booking-saga/services/booking-service/internal/domain"
)

type bookingCreator interface {
	RequestBooking(ctx context.Context, riderID, pickup, dropoff string) (domain.Booking, error)
}

type bookingFinder interface {
	FindByID(ctx context.Context, id string) (domain.Booking, error)
}

type Handler struct {
	creator bookingCreator
	finder  bookingFinder
	mux     *http.ServeMux
}

func NewHandler(creator bookingCreator, finder bookingFinder) *Handler {
	h := &Handler{creator: creator, finder: finder, mux: http.NewServeMux()}
	h.mux.HandleFunc("POST /bookings", h.createBooking)
	h.mux.HandleFunc("GET /bookings/{id}", h.getBooking)
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

type createBookingRequest struct {
	RiderID string `json:"rider_id"`
	Pickup  string `json:"pickup"`
	Dropoff string `json:"dropoff"`
}

func (h *Handler) createBooking(w http.ResponseWriter, r *http.Request) {
	var req createBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	b, err := h.creator.RequestBooking(r.Context(), req.RiderID, req.Pickup, req.Dropoff)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(b)
}

func (h *Handler) getBooking(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	b, err := h.finder.FindByID(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(b)
}
