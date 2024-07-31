package repo

import (
	"errors"
)

type Plusofon struct {
	PlusofonToken string
	ClientID      string
}

func NewPlusofon(plusofonToken, clientID string) (*Plusofon, error) {
	if plusofonToken == "" || clientID == "" {
		return nil, errors.New("api token and client ID are required")
	}
	return &Plusofon{
		PlusofonToken: plusofonToken,
		ClientID:      clientID,
	}, nil
}
