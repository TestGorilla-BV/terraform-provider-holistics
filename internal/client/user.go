package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type User struct {
	ID                       int     `json:"id"`
	Email                    string  `json:"email"`
	Name                     *string `json:"name,omitempty"`
	Initials                 string  `json:"initials"`
	Role                     string  `json:"role"`
	IsDeleted                bool    `json:"is_deleted"`
	IsActivated              bool    `json:"is_activated"`
	HasAuthenticationToken   bool    `json:"has_authentication_token"`
	AllowAuthenticationToken bool    `json:"allow_authentication_token"`
	EnableExportData         bool    `json:"enable_export_data"`
	CurrentSignInAt          *string `json:"current_sign_in_at,omitempty"`
	LastSignInAt             *string `json:"last_sign_in_at,omitempty"`
	CreatedAt                string  `json:"created_at"`
	Title                    *string `json:"title,omitempty"`
	JobTitle                 *string `json:"job_title,omitempty"`
	GroupIDs                 []int   `json:"group_ids,omitempty"`
}

type ListUsersFilter struct {
	SearchTerm             string
	Statuses               []string
	IDs                    []int
	Role                   string
	HasAuthenticationToken *bool
	ExcludeDeleted         *bool
}

type listUsersResponse struct {
	Users   []User `json:"users"`
	Cursors struct {
		Next *string `json:"next"`
	} `json:"cursors"`
}

func (c *Client) GetCurrentUser(ctx context.Context) (*User, error) {
	var out struct {
		User User `json:"user"`
	}
	if err := c.Do(ctx, http.MethodGet, "/users/me", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.User, nil
}

// ListUsers fetches all users matching the filter, paging until the cursor is empty.
func (c *Client) ListUsers(ctx context.Context, filter ListUsersFilter) ([]User, error) {
	var users []User
	var after string
	for {
		q := url.Values{}
		q.Set("limit", "100")
		if after != "" {
			q.Set("after", after)
		}
		if filter.SearchTerm != "" {
			q.Set("search_term", filter.SearchTerm)
		}
		if filter.Role != "" {
			q.Set("role", filter.Role)
		}
		if filter.HasAuthenticationToken != nil {
			q.Set("has_authentication_token", strconv.FormatBool(*filter.HasAuthenticationToken))
		}
		if filter.ExcludeDeleted != nil {
			q.Set("exclude_deleted", strconv.FormatBool(*filter.ExcludeDeleted))
		}
		for _, s := range filter.Statuses {
			q.Add("statuses[]", s)
		}
		for _, id := range filter.IDs {
			q.Add("ids[]", strconv.Itoa(id))
		}

		var page listUsersResponse
		if err := c.Do(ctx, http.MethodGet, "/users", q, nil, &page); err != nil {
			return nil, err
		}
		users = append(users, page.Users...)
		if page.Cursors.Next == nil || *page.Cursors.Next == "" {
			break
		}
		after = *page.Cursors.Next
	}
	return users, nil
}

// GetUser fetches a single user by ID. The API doesn't expose GET /users/{id};
// this filters the list endpoint by id.
func (c *Client) GetUser(ctx context.Context, id int) (*User, error) {
	users, err := c.ListUsers(ctx, ListUsersFilter{IDs: []int{id}})
	if err != nil {
		return nil, err
	}
	for i := range users {
		if users[i].ID == id {
			return &users[i], nil
		}
	}
	return nil, &APIError{StatusCode: http.StatusNotFound, Message: fmt.Sprintf("user %d not found", id)}
}

// GetUserByEmail looks up a user by exact email match. Includes soft-deleted
// users so callers can detect re-invitation of a previously-deleted user.
// Returns a NotFound APIError if no match exists.
func (c *Client) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	users, err := c.ListUsers(ctx, ListUsersFilter{SearchTerm: email})
	if err != nil {
		return nil, err
	}
	target := strings.ToLower(email)
	for i := range users {
		if strings.ToLower(users[i].Email) == target {
			return &users[i], nil
		}
	}
	return nil, &APIError{StatusCode: http.StatusNotFound, Message: fmt.Sprintf("user with email %q not found", email)}
}

type InviteUserInput struct {
	Email                    string
	Role                     string
	AllowAuthenticationToken *bool
	EnableExportData         *bool
	GroupIDs                 []int
	Message                  *string
}

// InviteUser sends an invitation to a single email. The Holistics API invites
// a batch and returns an async job, so this helper waits for the user record
// to appear in the list endpoint and returns it.
func (c *Client) InviteUser(ctx context.Context, input InviteUserInput) (*User, error) {
	body := map[string]any{
		"emails": []string{input.Email},
		"role":   input.Role,
	}
	if input.AllowAuthenticationToken != nil {
		body["allow_authentication_token"] = *input.AllowAuthenticationToken
	}
	if input.EnableExportData != nil {
		body["enable_export_data"] = *input.EnableExportData
	}
	if len(input.GroupIDs) > 0 {
		body["group_ids"] = input.GroupIDs
	}
	if input.Message != nil {
		body["message"] = *input.Message
	}

	if err := c.Do(ctx, http.MethodPost, "/users/invite", nil, body, nil); err != nil {
		return nil, err
	}

	// The user record is typically materialized synchronously even though
	// the email send is async. Retry briefly for eventual consistency.
	deadline := time.Now().Add(15 * time.Second)
	for {
		u, err := c.GetUserByEmail(ctx, input.Email)
		if err == nil {
			return u, nil
		}
		if !IsNotFound(err) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("invited %q but user did not appear in list within 15s", input.Email)
		}
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

type UpdateUserInput struct {
	Name                     *string
	Title                    *string
	JobTitle                 *string
	Role                     *string
	AllowAuthenticationToken *bool
	EnableExportData         *bool
	GroupIDs                 *[]int
}

// UpdateUser sends a PUT to /users/{id}. Only non-nil fields are sent.
func (c *Client) UpdateUser(ctx context.Context, id int, input UpdateUserInput) (*User, error) {
	user := map[string]any{}
	if input.Name != nil {
		user["name"] = *input.Name
	}
	if input.Title != nil {
		user["title"] = *input.Title
	}
	if input.JobTitle != nil {
		user["job_title"] = *input.JobTitle
	}
	if input.Role != nil {
		user["role"] = *input.Role
	}
	if input.AllowAuthenticationToken != nil {
		user["allow_authentication_token"] = *input.AllowAuthenticationToken
	}
	if input.EnableExportData != nil {
		user["enable_export_data"] = *input.EnableExportData
	}
	if input.GroupIDs != nil {
		user["group_ids"] = *input.GroupIDs
	}
	body := map[string]any{"user": user}

	var out User
	if err := c.Do(ctx, http.MethodPut, fmt.Sprintf("/users/%d", id), nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SoftDeleteUser flips is_deleted=true on the user.
func (c *Client) SoftDeleteUser(ctx context.Context, id int) error {
	return c.Do(ctx, http.MethodDelete, fmt.Sprintf("/users/%d", id), nil, nil, nil)
}

// RestoreUser reverses a soft-delete.
func (c *Client) RestoreUser(ctx context.Context, id int) error {
	return c.Do(ctx, http.MethodPost, fmt.Sprintf("/users/%d/restore", id), nil, nil, nil)
}
