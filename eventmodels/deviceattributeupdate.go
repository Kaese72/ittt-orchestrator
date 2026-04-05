package eventmodels

// UpdatedAttribute is an attribute state as published by the device-store.
type UpdatedAttribute struct {
	Name    string   `json:"name"`
	Boolean *bool    `json:"boolean-state,omitempty"`
	Numeric *float32 `json:"numeric-state,omitempty"`
	Text    *string  `json:"string-state,omitempty"`
}

// DeviceAttributeUpdate is the event published by the device-store when
// one or more attributes of a device change.
type DeviceAttributeUpdate struct {
	DeviceID   int                `json:"device-id"`
	Attributes []UpdatedAttribute `json:"attributes"`
}
