package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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
