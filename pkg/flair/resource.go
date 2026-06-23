package flair

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Relationship is a JSON:API relationship entry within a resource object.
type Relationship struct {
	Links map[string]string `json:"links,omitempty"`
	Data  json.RawMessage   `json:"data,omitempty"`
}

// Related returns the "related" link, which fetches the related resource(s).
func (r Relationship) Related() string { return r.Links["related"] }

// Resource is a typed JSON:API resource object.
type Resource[T any] struct {
	ID            string
	Type          string
	Attributes    T
	Relationships map[string]Relationship
}

// Rel returns the named relationship, or a zero Relationship if absent.
func (r Resource[T]) Rel(name string) Relationship {
	return r.Relationships[name]
}

// PageInfo holds JSON:API pagination metadata returned alongside a collection.
type PageInfo struct {
	TotalCount int    // total number of matching resources across all pages
	PageSize   int    // page size used for this response
	NextURL    string // URL of the next page; empty on the last page
}

// Collection is a JSON:API resource collection with pagination metadata.
type Collection[T any] struct {
	Resources []Resource[T]
	Page      PageInfo
}

// Ptr returns a pointer to v, for use in patch structs where nil means "leave unchanged".
func Ptr[T any](v T) *T { return &v }

// APIError is returned when the Flair API responds with one or more JSON:API error objects.
type APIError struct {
	Errors []APIErrorDetail
}

// APIErrorDetail is a single error entry in a JSON:API error response.
type APIErrorDetail struct {
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

func (e *APIError) Error() string {
	msgs := make([]string, len(e.Errors))
	for i, detail := range e.Errors {
		msgs[i] = fmt.Sprintf("%d %s: %s", detail.Status, detail.Title, detail.Detail)
	}
	return "flair: " + strings.Join(msgs, "; ")
}

// ListRelated fetches the resource collection at the given URL, which is typically obtained
// from Relationship.Related(). It follows full https:// URLs or /api/... paths.
func ListRelated[T any](client *Client, url string) (Collection[T], error) {
	raw, err := client.list(url, nil)
	if err != nil {
		return Collection[T]{}, err
	}
	return decodeCollection[T](raw)
}

// GetLinked fetches the single resource at the given URL, which is typically obtained from
// Relationship.Related(). It follows full https:// URLs or /api/... paths.
func GetLinked[T any](client *Client, url string) (Resource[T], error) {
	raw, err := client.get(url, nil)
	if err != nil {
		return Resource[T]{}, err
	}
	return decodeResource[T](raw.Data)
}

// --- internal JSON:API wire types ---

type rawResource struct {
	ID            string                  `json:"id"`
	Type          string                  `json:"type"`
	Attributes    json.RawMessage         `json:"attributes"`
	Relationships map[string]Relationship `json:"relationships,omitempty"`
}

type rawEnvelope struct {
	Data   rawResource      `json:"data"`
	Errors []APIErrorDetail `json:"errors,omitempty"`
}

type rawCollection struct {
	Data   []rawResource    `json:"data"`
	Meta   rawMeta          `json:"meta,omitempty"`
	Links  rawLinks         `json:"links,omitempty"`
	Errors []APIErrorDetail `json:"errors,omitempty"`
}

type rawMeta struct {
	TotalCount int `json:"total-count"`
	PageSize   int `json:"page-size"`
}

type rawLinks struct {
	Next string `json:"next,omitempty"`
}

func decodeResource[T any](r rawResource) (Resource[T], error) {
	var attributes T
	if len(r.Attributes) > 0 {
		if err := json.Unmarshal(r.Attributes, &attributes); err != nil {
			return Resource[T]{}, fmt.Errorf("flair: decode %s attributes: %w", r.Type, err)
		}
	}
	return Resource[T]{
		ID:            r.ID,
		Type:          r.Type,
		Attributes:    attributes,
		Relationships: r.Relationships,
	}, nil
}

func decodeCollection[T any](raw rawCollection) (Collection[T], error) {
	resources := make([]Resource[T], 0, len(raw.Data))
	for _, r := range raw.Data {
		resource, err := decodeResource[T](r)
		if err != nil {
			return Collection[T]{}, err
		}
		resources = append(resources, resource)
	}
	return Collection[T]{
		Resources: resources,
		Page: PageInfo{
			TotalCount: raw.Meta.TotalCount,
			PageSize:   raw.Meta.PageSize,
			NextURL:    raw.Links.Next,
		},
	}, nil
}

// --- JSON:API request body helpers ---

type patchBody struct {
	Data patchData `json:"data"`
}

type patchData struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes any    `json:"attributes"`
}

type createBody struct {
	Data createData `json:"data"`
}

type createData struct {
	Type          string                `json:"type"`
	Attributes    any                   `json:"attributes"`
	Relationships map[string]relWrapper `json:"relationships,omitempty"`
}

type relWrapper struct {
	Data relItem `json:"data"`
}

type relItem struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}
