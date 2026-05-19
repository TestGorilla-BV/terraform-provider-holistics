// Package mockserver provides an in-memory HTTP server that simulates the
// subset of the Holistics API exercised by the provider's acceptance tests.
// It does NOT aim for full API fidelity — only the request/response shape that
// the provider depends on.
package mockserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Server wraps an httptest.Server and maintains in-memory state per resource.
type Server struct {
	*httptest.Server

	mu              sync.Mutex
	nextID          int
	groups          map[int]*group
	groupMembership map[int]map[int]bool
	userAttributes  map[int]*userAttribute
	dataSchedules   map[int]map[string]any
}

type group struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type userAttribute struct {
	ID                int     `json:"id"`
	Name              string  `json:"name"`
	AttributeType     string  `json:"attribute_type"`
	Label             string  `json:"label"`
	Description       *string `json:"description,omitempty"`
	IsSystemAttribute bool    `json:"is_system_attribute"`
}

// New starts a mock server and returns it. Call srv.Close when done.
func New() *Server {
	s := &Server{
		nextID:          1,
		groups:          map[int]*group{},
		groupMembership: map[int]map[int]bool{},
		userAttributes:  map[int]*userAttribute{},
		dataSchedules:   map[int]map[string]any{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/groups", s.handleGroupsCollection)
	mux.HandleFunc("/api/v2/groups/", s.handleGroupsItem)
	mux.HandleFunc("/api/v2/user_attributes", s.handleUserAttributesCollection)
	mux.HandleFunc("/api/v2/user_attributes/", s.handleUserAttributesItem)
	mux.HandleFunc("/api/v2/data_schedules", s.handleDataSchedulesCollection)
	mux.HandleFunc("/api/v2/data_schedules/", s.handleDataSchedulesItem)
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Holistics-Key") == "" {
			respondError(w, http.StatusUnauthorized, "AuthError", "missing api key")
			return
		}
		mux.ServeHTTP(w, r)
	}))
	return s
}

// BaseURL returns the URL to pass to HOLISTICS_BASE_URL.
func (s *Server) BaseURL() string {
	return s.URL + "/api/v2"
}

func (s *Server) nextResourceID() int {
	id := s.nextID
	s.nextID++
	return id
}

func respondJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func respondError(w http.ResponseWriter, status int, errType, msg string) {
	respondJSON(w, status, map[string]any{
		"type":    errType,
		"message": msg,
	})
}

// ----- Groups -----

func (s *Server) handleGroupsCollection(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.Method {
	case http.MethodPost:
		var body struct {
			Group struct {
				Name string `json:"name"`
			} `json:"group"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondError(w, http.StatusBadRequest, "InvalidParameterError", err.Error())
			return
		}
		g := &group{ID: s.nextResourceID(), Name: body.Group.Name}
		s.groups[g.ID] = g
		respondJSON(w, http.StatusCreated, map[string]any{"group": s.groupResponse(g)})
	case http.MethodGet:
		gs := make([]any, 0, len(s.groups))
		for _, g := range s.groups {
			gs = append(gs, s.groupResponse(g))
		}
		respondJSON(w, http.StatusOK, map[string]any{"groups": gs, "users": map[string]any{}, "cursors": map[string]any{"next": nil}})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

var groupItemRe = regexp.MustCompile(`^/api/v2/groups/(\d+)(?:/(add_user|remove_user)/(\d+))?$`)

func (s *Server) handleGroupsItem(w http.ResponseWriter, r *http.Request) {
	m := groupItemRe.FindStringSubmatch(r.URL.Path)
	if m == nil {
		http.NotFound(w, r)
		return
	}
	id, _ := strconv.Atoi(m[1])

	s.mu.Lock()
	defer s.mu.Unlock()

	if m[2] != "" {
		// /groups/{id}/(add_user|remove_user)/{uid}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		uid, _ := strconv.Atoi(m[3])
		mem := s.groupMembership[id]
		if mem == nil {
			mem = map[int]bool{}
			s.groupMembership[id] = mem
		}
		if m[2] == "add_user" {
			mem[uid] = true
		} else {
			delete(mem, uid)
		}
		respondJSON(w, http.StatusOK, map[string]any{"message": "ok"})
		return
	}

	g, ok := s.groups[id]
	if !ok {
		respondError(w, http.StatusNotFound, "NotFoundError", "group not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, http.StatusOK, map[string]any{"group": s.groupResponse(g)})
	case http.MethodPut:
		var body struct {
			Group struct {
				Name string `json:"name"`
			} `json:"group"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondError(w, http.StatusBadRequest, "InvalidParameterError", err.Error())
			return
		}
		if body.Group.Name != "" {
			g.Name = body.Group.Name
		}
		respondJSON(w, http.StatusOK, map[string]any{"group": s.groupResponse(g)})
	case http.MethodDelete:
		delete(s.groups, id)
		delete(s.groupMembership, id)
		respondJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) groupResponse(g *group) map[string]any {
	uids := make([]int, 0, len(s.groupMembership[g.ID]))
	for uid := range s.groupMembership[g.ID] {
		uids = append(uids, uid)
	}
	return map[string]any{"id": g.ID, "name": g.Name, "user_ids": uids}
}

// ----- User Attributes -----

func (s *Server) handleUserAttributesCollection(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.Method {
	case http.MethodPost:
		var body struct {
			UserAttribute userAttribute `json:"user_attribute"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondError(w, http.StatusBadRequest, "InvalidParameterError", err.Error())
			return
		}
		ua := body.UserAttribute
		ua.ID = s.nextResourceID()
		s.userAttributes[ua.ID] = &ua
		respondJSON(w, http.StatusOK, map[string]any{"user_attribute": ua, "status": "created"})
	case http.MethodGet:
		uas := make([]any, 0, len(s.userAttributes))
		for _, ua := range s.userAttributes {
			uas = append(uas, ua)
		}
		respondJSON(w, http.StatusOK, map[string]any{"user_attributes": uas, "cursors": map[string]any{"next": nil}})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUserAttributesItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/api/v2/user_attributes/"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ua, ok := s.userAttributes[id]
	if !ok {
		respondError(w, http.StatusNotFound, "NotFoundError", "user_attribute not found")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var body struct {
			UserAttribute userAttribute `json:"user_attribute"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondError(w, http.StatusBadRequest, "InvalidParameterError", err.Error())
			return
		}
		body.UserAttribute.ID = ua.ID
		s.userAttributes[id] = &body.UserAttribute
		respondJSON(w, http.StatusOK, map[string]any{"user_attribute": body.UserAttribute})
	case http.MethodDelete:
		delete(s.userAttributes, id)
		respondJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// ----- Data Schedules -----

func (s *Server) handleDataSchedulesCollection(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		DataSchedule map[string]any `json:"data_schedule"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "InvalidParameterError", err.Error())
		return
	}
	id := s.nextResourceID()
	body.DataSchedule["id"] = id
	s.dataSchedules[id] = body.DataSchedule
	respondJSON(w, http.StatusCreated, map[string]any{"data_schedule": body.DataSchedule})
}

func (s *Server) handleDataSchedulesItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/api/v2/data_schedules/"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ds, ok := s.dataSchedules[id]
	if !ok {
		respondError(w, http.StatusNotFound, "NotFoundError", "data_schedule not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, http.StatusOK, map[string]any{"data_schedule": ds})
	case http.MethodPut:
		var body struct {
			DataSchedule map[string]any `json:"data_schedule"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondError(w, http.StatusBadRequest, "InvalidParameterError", err.Error())
			return
		}
		body.DataSchedule["id"] = id
		s.dataSchedules[id] = body.DataSchedule
		respondJSON(w, http.StatusOK, map[string]any{"data_schedule": body.DataSchedule})
	case http.MethodDelete:
		delete(s.dataSchedules, id)
		respondJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
