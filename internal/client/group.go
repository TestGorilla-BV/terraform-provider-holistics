package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type Group struct {
	ID      *int   `json:"id,omitempty"`
	Name    string `json:"name"`
	UserIDs []int  `json:"user_ids,omitempty"`
}

type groupEnvelope struct {
	Group Group `json:"group"`
}

type groupInput struct {
	Name string `json:"name"`
}

type groupInputEnvelope struct {
	Group groupInput `json:"group"`
}

func (c *Client) CreateGroup(ctx context.Context, name string) (*Group, error) {
	body := groupInputEnvelope{Group: groupInput{Name: name}}
	var out groupEnvelope
	if err := c.Do(ctx, http.MethodPost, "/groups", nil, body, &out); err != nil {
		return nil, err
	}
	c.groupsCache.invalidate()
	return &out.Group, nil
}

func (c *Client) GetGroup(ctx context.Context, id int) (*Group, error) {
	var out groupEnvelope
	if err := c.Do(ctx, http.MethodGet, fmt.Sprintf("/groups/%d", id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out.Group, nil
}

func (c *Client) UpdateGroup(ctx context.Context, id int, name string) (*Group, error) {
	body := groupInputEnvelope{Group: groupInput{Name: name}}
	var out groupEnvelope
	if err := c.Do(ctx, http.MethodPut, fmt.Sprintf("/groups/%d", id), nil, body, &out); err != nil {
		return nil, err
	}
	c.groupsCache.invalidate()
	return &out.Group, nil
}

func (c *Client) DeleteGroup(ctx context.Context, id int) error {
	err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("/groups/%d", id), nil, nil, nil)
	if err == nil {
		c.groupsCache.invalidate()
	}
	return err
}

func (c *Client) AddGroupUser(ctx context.Context, groupID, userID int) error {
	err := c.Do(ctx, http.MethodPost, fmt.Sprintf("/groups/%d/add_user/%d", groupID, userID), nil, nil, nil)
	if err == nil {
		c.groupsCache.invalidate()
	}
	return err
}

func (c *Client) RemoveGroupUser(ctx context.Context, groupID, userID int) error {
	err := c.Do(ctx, http.MethodPost, fmt.Sprintf("/groups/%d/remove_user/%d", groupID, userID), nil, nil, nil)
	if err == nil {
		c.groupsCache.invalidate()
	}
	return err
}

type listGroupsResponse struct {
	Groups  []Group `json:"groups"`
	Cursors struct {
		Next *string `json:"next"`
	} `json:"cursors"`
}

// ListGroups paginates /groups and returns the full list with user_ids
// populated. Cached in-memory for the client's cache TTL — multiple calls
// within a single terraform run only hit the API once.
func (c *Client) ListGroups(ctx context.Context) ([]Group, error) {
	return c.groupsCache.get(ctx, func(ctx context.Context) ([]Group, error) {
		var groups []Group
		var after string
		for {
			q := url.Values{}
			q.Set("limit", "100")
			q.Set("include_users", "true")
			if after != "" {
				q.Set("after", after)
			}
			var page listGroupsResponse
			if err := c.Do(ctx, http.MethodGet, "/groups", q, nil, &page); err != nil {
				return nil, err
			}
			groups = append(groups, page.Groups...)
			if page.Cursors.Next == nil || *page.Cursors.Next == "" {
				break
			}
			after = *page.Cursors.Next
		}
		return groups, nil
	})
}

// GetGroupByName looks up a group by exact (case-sensitive) name match,
// reading from the cached group list. Returns a NotFound APIError if no
// group has that name, or a generic error if multiple groups share the name.
func (c *Client) GetGroupByName(ctx context.Context, name string) (*Group, error) {
	groups, err := c.ListGroups(ctx)
	if err != nil {
		return nil, err
	}
	var matches []Group
	for i := range groups {
		if groups[i].Name == name {
			matches = append(matches, groups[i])
		}
	}
	switch len(matches) {
	case 0:
		return nil, &APIError{StatusCode: http.StatusNotFound, Message: fmt.Sprintf("group with name %q not found", name)}
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("group name %q is ambiguous: %d groups share it; import by integer ID instead", name, len(matches))
	}
}

// GetGroupWithUsers fetches a group including the populated user_ids list.
func (c *Client) GetGroupWithUsers(ctx context.Context, id int) (*Group, error) {
	q := url.Values{}
	q.Set("include_users", "true")
	var out groupEnvelope
	path := fmt.Sprintf("/groups/%d", id)
	if err := c.Do(ctx, http.MethodGet, path, q, nil, &out); err != nil {
		return nil, err
	}
	return &out.Group, nil
}
