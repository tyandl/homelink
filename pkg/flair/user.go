package flair

import "github.com/tyandl/homelink/pkg/flair/types"

// UserAttributes are the properties of a Flair user account.
type UserAttributes struct {
	Name             string                 `json:"name"`
	Email            string                 `json:"email,omitempty"`
	TemperatureScale types.TemperatureScale `json:"temperature-scale"`
	IsCelsius        bool                   `json:"is-celsius"`
	CreatedAt        string                 `json:"created-at,omitempty"`
	UpdatedAt        string                 `json:"updated-at,omitempty"`
}

// GetMe returns the profile of the authenticated user.
func (client *Client) GetMe() (Resource[UserAttributes], error) {
	raw, err := client.get("/api/me", nil)
	if err != nil {
		return Resource[UserAttributes]{}, err
	}
	return decodeResource[UserAttributes](raw.Data)
}
