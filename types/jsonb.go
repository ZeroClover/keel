package types

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSONB is stored as a JSON blob.
type JSONB map[string]interface{}

func (b JSONB) Value() (driver.Value, error) {
	j, err := json.Marshal(b)
	return j, err
}

func (b *JSONB) Scan(src interface{}) error {
	source, ok := src.([]byte)
	if !ok {
		return errors.New("type assertion .([]byte) failed.")
	}

	var i interface{}
	if err := json.Unmarshal(source, &i); err != nil {
		return err
	}

	if i == nil {
		return nil
	}

	*b, ok = i.(map[string]interface{})
	if !ok {
		return errors.New("type assertion .(map[string]interface{}) failed.")
	}

	return nil
}
