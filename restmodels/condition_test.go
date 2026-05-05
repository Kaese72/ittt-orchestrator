package restmodels_test

import (
	"time"

	"github.com/Kaese72/ittt-orchestrator/internal/devicestore"
)

// stubEvalContext satisfies restmodels.EvalContext for tests that only need Now().
type stubEvalContext struct {
	now   time.Time
	store map[int]map[string]devicestore.Attribute
}

func (s stubEvalContext) Now() time.Time { return s.now }
func (s stubEvalContext) GetDeviceAttribute(deviceID int, attributeName string) (*devicestore.Attribute, error) {
	if attrs, ok := s.store[deviceID]; ok {
		if attr, ok := attrs[attributeName]; ok {
			return &attr, nil
		}
	}
	return nil, nil
}
