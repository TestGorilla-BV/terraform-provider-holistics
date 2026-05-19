// Package mockserver provides an in-memory HTTP server that simulates the
// subset of the Holistics API exercised by the provider's acceptance tests.
// It does NOT aim for full API fidelity — only the request/response shape that
// the provider depends on.
package mockserver

import (
	"encoding/json"
	"fmt"
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
	dataAlerts      map[int]map[string]any
	shareableLinks  map[string]map[string]any
	users           map[int]*user

	// TruncateListUserDetails, when set, makes the /users list endpoint
	// omit title and job_title from each response item — even when the
	// stored user has them set. This reproduces a real Holistics API quirk
	// where the list view returns a subset of the fields PUT /users/{id}
	// returns. Tests that exercise this gap can flip this flag.
	TruncateListUserDetails bool
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

type user struct {
	ID                       int     `json:"id"`
	Email                    string  `json:"email"`
	Name                     *string `json:"name,omitempty"`
	Title                    *string `json:"title,omitempty"`
	JobTitle                 *string `json:"job_title,omitempty"`
	Initials                 string  `json:"initials"`
	Role                     string  `json:"role"`
	IsDeleted                bool    `json:"is_deleted"`
	IsActivated              bool    `json:"is_activated"`
	HasAuthenticationToken   bool    `json:"has_authentication_token"`
	AllowAuthenticationToken bool    `json:"allow_authentication_token"`
	EnableExportData         bool    `json:"enable_export_data"`
	CreatedAt                string  `json:"created_at"`
	GroupIDs                 []int   `json:"group_ids"`
}

// New starts a mock server and returns it. Call srv.Close when done.
func New() *Server {
	s := &Server{
		nextID:          1,
		groups:          map[int]*group{},
		groupMembership: map[int]map[int]bool{},
		userAttributes:  map[int]*userAttribute{},
		dataSchedules:   map[int]map[string]any{},
		dataAlerts:      map[int]map[string]any{},
		shareableLinks:  map[string]map[string]any{},
		users:           map[int]*user{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/groups", s.handleGroupsCollection)
	mux.HandleFunc("/api/v2/groups/", s.handleGroupsItem)
	mux.HandleFunc("/api/v2/user_attributes", s.handleUserAttributesCollection)
	mux.HandleFunc("/api/v2/user_attributes/", s.handleUserAttributesItem)
	mux.HandleFunc("/api/v2/data_schedules", s.handleDataSchedulesCollection)
	mux.HandleFunc("/api/v2/data_schedules/", s.handleDataSchedulesItem)
	mux.HandleFunc("/api/v2/data_alerts", s.handleDataAlertsCollection)
	mux.HandleFunc("/api/v2/data_alerts/", s.handleDataAlertsItem)
	mux.HandleFunc("/api/v2/shareable_links", s.handleShareableLinksCollection)
	mux.HandleFunc("/api/v2/shareable_links/", s.handleShareableLinksItem)
	mux.HandleFunc("/api/v2/users", s.handleUsersList)
	mux.HandleFunc("/api/v2/users/", s.handleUsersItem)
	mux.HandleFunc("/api/v2/users/invite", s.handleUsersInvite)
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

// SetUserFieldByEmail directly mutates a stored user's field without going
// through the API surface. Used by acceptance tests that need to simulate a
// server-side state change that didn't originate from the provider — e.g.
// "the user already had a title set in Holistics before we imported them".
// Only string-valued single fields are supported. Returns an error if the
// user is missing or the field is unsupported.
func (s *Server) SetUserFieldByEmail(email, field, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if u.Email == email {
			switch field {
			case "title":
				v := value
				u.Title = &v
				return nil
			case "job_title":
				v := value
				u.JobTitle = &v
				return nil
			default:
				return fmt.Errorf("unsupported field %q", field)
			}
		}
	}
	return fmt.Errorf("user with email %q not found", email)
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

// ----- Data Alerts -----

func (s *Server) handleDataAlertsCollection(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		DataAlert map[string]any `json:"data_alert"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "InvalidParameterError", err.Error())
		return
	}
	id := s.nextResourceID()
	body.DataAlert["id"] = id
	s.dataAlerts[id] = body.DataAlert
	respondJSON(w, http.StatusCreated, map[string]any{"data_alert": body.DataAlert})
}

func (s *Server) handleDataAlertsItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/api/v2/data_alerts/"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	da, ok := s.dataAlerts[id]
	if !ok {
		respondError(w, http.StatusNotFound, "NotFoundError", "data_alert not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, http.StatusOK, map[string]any{"data_alert": da})
	case http.MethodPut:
		var body struct {
			DataAlert map[string]any `json:"data_alert"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondError(w, http.StatusBadRequest, "InvalidParameterError", err.Error())
			return
		}
		body.DataAlert["id"] = id
		s.dataAlerts[id] = body.DataAlert
		respondJSON(w, http.StatusOK, map[string]any{"data_alert": body.DataAlert})
	case http.MethodDelete:
		delete(s.dataAlerts, id)
		respondJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// ----- Shareable Links -----

func (s *Server) handleShareableLinksCollection(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		ShareableLink map[string]any `json:"shareable_link"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "InvalidParameterError", err.Error())
		return
	}
	id := strconv.Itoa(s.nextResourceID())
	body.ShareableLink["id"] = id
	s.shareableLinks[id] = body.ShareableLink
	respondJSON(w, http.StatusCreated, map[string]any{"shareable_link": body.ShareableLink})
}

func (s *Server) handleShareableLinksItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v2/shareable_links/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sl, ok := s.shareableLinks[id]
	if !ok {
		respondError(w, http.StatusNotFound, "NotFoundError", "shareable_link not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, http.StatusOK, map[string]any{"shareable_link": sl})
	case http.MethodPut:
		var body struct {
			ShareableLink map[string]any `json:"shareable_link"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondError(w, http.StatusBadRequest, "InvalidParameterError", err.Error())
			return
		}
		body.ShareableLink["id"] = id
		s.shareableLinks[id] = body.ShareableLink
		respondJSON(w, http.StatusOK, map[string]any{"shareable_link": body.ShareableLink})
	case http.MethodDelete:
		delete(s.shareableLinks, id)
		respondJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// ----- Users -----

var userItemRe = regexp.MustCompile(`^/api/v2/users/(\d+)(?:/(restore))?$`)

func (s *Server) handleUsersList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	q := r.URL.Query()
	searchTerm := strings.ToLower(q.Get("search_term"))
	wantIDs := map[int]bool{}
	for _, raw := range q["ids[]"] {
		if id, err := strconv.Atoi(raw); err == nil {
			wantIDs[id] = true
		}
	}

	out := make([]*user, 0, len(s.users))
	for _, u := range s.users {
		if len(wantIDs) > 0 && !wantIDs[u.ID] {
			continue
		}
		if searchTerm != "" && !strings.Contains(strings.ToLower(u.Email), searchTerm) {
			continue
		}
		if s.TruncateListUserDetails {
			uc := *u
			uc.Title = nil
			uc.JobTitle = nil
			out = append(out, &uc)
		} else {
			out = append(out, u)
		}
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"users":    out,
		"counters": map[string]any{},
		"groups":   map[string]any{},
		"cursors":  map[string]any{"next": nil},
	})
}

func (s *Server) handleUsersInvite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Emails                   []string `json:"emails"`
		Role                     string   `json:"role"`
		AllowAuthenticationToken *bool    `json:"allow_authentication_token,omitempty"`
		EnableExportData         *bool    `json:"enable_export_data,omitempty"`
		GroupIDs                 []int    `json:"group_ids,omitempty"`
		Message                  *string  `json:"message,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "InvalidParameterError", err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, email := range body.Emails {
		// Reject if a non-deleted user with this email already exists.
		for _, u := range s.users {
			if !u.IsDeleted && strings.EqualFold(u.Email, email) {
				respondError(w, http.StatusUnprocessableEntity, "InvalidOperationError", "email already in use")
				return
			}
		}
		u := &user{
			ID:                       s.nextResourceID(),
			Email:                    email,
			Role:                     body.Role,
			Initials:                 strings.ToUpper(string(email[0])),
			IsActivated:              false,
			CreatedAt:                "2026-01-01T00:00:00Z",
			AllowAuthenticationToken: body.AllowAuthenticationToken != nil && *body.AllowAuthenticationToken,
			EnableExportData:         body.EnableExportData != nil && *body.EnableExportData,
			GroupIDs:                 append([]int(nil), body.GroupIDs...),
		}
		s.users[u.ID] = u
	}
	respondJSON(w, http.StatusOK, map[string]any{"job": map[string]any{"id": s.nextResourceID()}})
}

func (s *Server) handleUsersItem(w http.ResponseWriter, r *http.Request) {
	// Catch /users/invite before it gets here.
	if r.URL.Path == "/api/v2/users/invite" {
		s.handleUsersInvite(w, r)
		return
	}
	m := userItemRe.FindStringSubmatch(r.URL.Path)
	if m == nil {
		http.NotFound(w, r)
		return
	}
	id, _ := strconv.Atoi(m[1])

	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		respondError(w, http.StatusNotFound, "NotFoundError", "user not found")
		return
	}

	if m[2] == "restore" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		u.IsDeleted = false
		respondJSON(w, http.StatusOK, map[string]any{"message": "restored"})
		return
	}

	switch r.Method {
	case http.MethodPut:
		var body struct {
			User map[string]any `json:"user"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondError(w, http.StatusBadRequest, "InvalidParameterError", err.Error())
			return
		}
		applyUserUpdate(u, body.User)
		respondJSON(w, http.StatusOK, u)
	case http.MethodDelete:
		u.IsDeleted = true
		respondJSON(w, http.StatusOK, map[string]any{"message": "soft-deleted"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func applyUserUpdate(u *user, body map[string]any) {
	if v, ok := body["name"]; ok {
		if s, ok := v.(string); ok {
			u.Name = &s
		}
	}
	if v, ok := body["title"]; ok {
		if s, ok := v.(string); ok {
			u.Title = &s
		}
	}
	if v, ok := body["job_title"]; ok {
		if s, ok := v.(string); ok {
			u.JobTitle = &s
		}
	}
	if v, ok := body["role"]; ok {
		if s, ok := v.(string); ok {
			u.Role = s
		}
	}
	if v, ok := body["allow_authentication_token"]; ok {
		if b, ok := v.(bool); ok {
			u.AllowAuthenticationToken = b
		}
	}
	if v, ok := body["enable_export_data"]; ok {
		if b, ok := v.(bool); ok {
			u.EnableExportData = b
		}
	}
	if v, ok := body["group_ids"]; ok {
		if arr, ok := v.([]any); ok {
			ids := make([]int, 0, len(arr))
			for _, n := range arr {
				if f, ok := n.(float64); ok {
					ids = append(ids, int(f))
				}
			}
			u.GroupIDs = ids
		}
	}
}
